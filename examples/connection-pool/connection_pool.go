package test

import (
	"context"
	"errors"
	"fmt"
	"log"
	"sync"
	"time"

	concurrentmap "github.com/Dsouza10082/ConcurrentOrderedMap"
	_ "github.com/jackc/pgx/v5/stdlib"
	"github.com/jmoiron/sqlx"
)

type PooledConnection struct {
	DB        *sqlx.DB
	CreatedAt time.Time
	LastUsed  time.Time
	ID        string
	InUse     bool
}

type ConnectionInstance struct {
	mu                  sync.RWMutex
	created             time.Time
	connections         *concurrentmap.ConcurrentOrderedMap[string, *PooledConnection]
	availableCount      int32
	config              *ConfigInstance
	expireConnectionsCh chan bool
	renewConnectionsCh  chan bool
	stopCh              chan bool
	wg                  sync.WaitGroup
	connectionCounter   int
}

var (
	instance *ConnectionInstance
	once     sync.Once
)

func GetConnectionInstance() *ConnectionInstance {
	once.Do(func() {
		instance = newConnectionInstance()
	})
	return instance
}

func newConnectionInstance() *ConnectionInstance {
	config := NewConfigInstance()

	connInst := &ConnectionInstance{
		created:             time.Now(),
		connections:         concurrentmap.NewConcurrentOrderedMapWithCapacity[string, *PooledConnection](config.MaxConnections),
		availableCount:      0,
		config:              config,
		expireConnectionsCh: make(chan bool, 1),
		renewConnectionsCh:  make(chan bool, 1),
		stopCh:              make(chan bool),
		connectionCounter:   0,
	}

	for i := 0; i < config.MinConnections; i++ {
		conn, err := connInst.createConnection()
		if err != nil {
			log.Printf("Error creating initial connection %d: %v", i, err)
			continue
		}
		connInst.connections.Set(conn.ID, conn)
		connInst.availableCount++
	}

	connInst.wg.Add(1)
	go connInst.maintainPool()

	log.Printf("Connection pool initialized with %d connections", connInst.connections.Len())
	return connInst
}

func (c *ConnectionInstance) createConnection() (*PooledConnection, error) {
	db, err := sqlx.Open("pgx", c.config.DatabaseHost)
	if err != nil {
		return nil, fmt.Errorf("error opening connection: %w", err)
	}

	db.SetMaxOpenConns(1)
	db.SetMaxIdleConns(1)
	db.SetConnMaxLifetime(0)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := db.PingContext(ctx); err != nil {
		db.Close()
		return nil, fmt.Errorf("error pinging connection: %w", err)
	}

	c.mu.Lock()
	c.connectionCounter++
	id := fmt.Sprintf("conn_%d_%d", c.connectionCounter, time.Now().UnixNano())
	c.mu.Unlock()

	return &PooledConnection{
		DB:        db,
		CreatedAt: time.Now(),
		LastUsed:  time.Now(),
		ID:        id,
		InUse:     false,
	}, nil
}

func (c *ConnectionInstance) IsConnectionValid(conn *PooledConnection) bool {
	if conn == nil || conn.DB == nil {
		return false
	}

	connectionLifeTime := time.Duration(c.config.ConnectionLifeTime) * time.Second
	if time.Since(conn.CreatedAt) > connectionLifeTime {
		log.Printf("Connection %s expired due to lifetime", conn.ID)
		return false
	}

	maxIdleTime := 5 * time.Minute
	if !conn.InUse && time.Since(conn.LastUsed) > maxIdleTime {
		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		defer cancel()

		if err := conn.DB.PingContext(ctx); err != nil {
			log.Printf("Connection %s failed ping: %v", conn.ID, err)
			return false
		}
	}

	return true
}

func (c *ConnectionInstance) maintainPool() {
	defer c.wg.Done()

	ticker := time.NewTicker(10 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-c.stopCh:
			log.Println("Stopping pool maintenance")
			return

		case <-ticker.C:
			c.performMaintenance()

		case <-c.expireConnectionsCh:
			log.Println("Request to expire old connections")
			c.expireOldConnections()
			c.ensureMinimumConnections()

		case <-c.renewConnectionsCh:
			log.Println("Request to renew connections")
			c.ensureMaximumConnections()
		}
	}
}

func (c *ConnectionInstance) performMaintenance() {
	expiredCount := 0
	validCount := 0
	inUseCount := 0
	availableCount := 0

	orderedPairs := c.connections.GetOrderedV2()
	toRemove := make([]string, 0)

	for _, pair := range orderedPairs {
		conn := pair.Value

		if !c.IsConnectionValid(conn) {
			conn.DB.Close()
			toRemove = append(toRemove, pair.Key)
			expiredCount++
		} else {
			validCount++
			if conn.InUse {
				inUseCount++
			} else {
				availableCount++
			}
		}
	}

	for _, id := range toRemove {
		c.connections.Delete(id)
	}

	c.availableCount = int32(availableCount)

	log.Printf("Pool Status - Total: %d, Valid: %d, Available: %d, In use: %d, Expired: %d",
		c.connections.Len(), validCount, availableCount, inUseCount, expiredCount)

	if validCount < c.config.MinConnections {
		go c.ensureMinimumConnections()
	}
}

func (c *ConnectionInstance) expireOldConnections() {
	connectionLifeTime := time.Duration(c.config.ConnectionLifeTime) * time.Second
	toRemove := make([]string, 0)

	orderedPairs := c.connections.GetOrderedV2()

	for _, pair := range orderedPairs {
		conn := pair.Value
		if time.Since(conn.CreatedAt) > connectionLifeTime {
			conn.DB.Close()
			toRemove = append(toRemove, pair.Key)
			log.Printf("Connection %s forcibly expired", conn.ID)
		}
	}

	for _, id := range toRemove {
		c.connections.Delete(id)
	}
}

func (c *ConnectionInstance) ensureMinimumConnections() {
	currentCount := c.connections.Len()
	needed := c.config.MinConnections - currentCount

	if needed <= 0 {
		return
	}

	log.Printf("Creating %d connections to reach minimum", needed)

	for i := 0; i < needed; i++ {
		conn, err := c.createConnection()
		if err != nil {
			log.Printf("Error creating replacement connection: %v", err)
			continue
		}

		c.connections.Set(conn.ID, conn)
		c.availableCount++
		log.Printf("New connection %s added to pool", conn.ID)
	}
}

func (c *ConnectionInstance) ensureMaximumConnections() {
	// Lock to prevent race conditions when checking and adding connections
	c.mu.Lock()
	defer c.mu.Unlock()
	
	currentCount := c.connections.Len()
	needed := c.config.MaxConnections - currentCount

	if needed <= 0 {
		log.Printf("Pool already at maximum (%d connections)", currentCount)
		return
	}

	log.Printf("Creating %d additional connections", needed)

	for i := 0; i < needed; i++ {
		// Check again inside the loop to ensure we don't exceed max
		currentCount = c.connections.Len()
		if currentCount >= c.config.MaxConnections {
			log.Printf("Reached maximum connections during creation")
			break
		}
		
		// Temporarily unlock for connection creation (which may take time)
		c.mu.Unlock()
		conn, err := c.createConnection()
		c.mu.Lock()
		
		if err != nil {
			log.Printf("Error creating additional connection: %v", err)
			continue
		}

		// Double-check we haven't exceeded max while creating the connection
		currentCount = c.connections.Len()
		if currentCount >= c.config.MaxConnections {
			conn.DB.Close()
			log.Printf("Maximum connections reached, closing excess connection %s", conn.ID)
			break
		}

		c.connections.Set(conn.ID, conn)
		c.availableCount++
		log.Printf("Additional connection %s created", conn.ID)
	}
}

func (c *ConnectionInstance) GetConnection(ctx context.Context) (*sqlx.DB, error) {
	c.mu.RLock()
	maxConnections := c.config.MaxConnections
	c.mu.RUnlock()

	orderedPairs := c.connections.GetOrderedV2()

	for _, pair := range orderedPairs {
		conn := pair.Value

		if !conn.InUse {
			err := c.connections.UpdateWithPointer(pair.Key, func(v **PooledConnection) error {
				if (*v).InUse {
					return errors.New("connection already in use")
				}

				if !c.IsConnectionValid(*v) {
					return errors.New("invalid connection")
				}

				(*v).InUse = true
				(*v).LastUsed = time.Now()
				c.availableCount--
				return nil
			})

			if err == nil {
				log.Printf("Connection %s obtained from pool", conn.ID)
				return conn.DB, nil
			}
		}
	}

	// Try to create a new connection if below max
	currentCount := c.connections.Len()
	if currentCount < maxConnections {
		c.mu.Lock()
		// Double-check after acquiring lock
		currentCount = c.connections.Len()
		if currentCount < c.config.MaxConnections {
			// Keep the lock while creating and adding the connection
			// to prevent race conditions
			
			// Temporarily unlock for connection creation (which may take time)
			c.mu.Unlock()
			newConn, err := c.createConnection()
			c.mu.Lock()
			
			if err != nil {
				c.mu.Unlock()
				return nil, fmt.Errorf("pool exhausted and could not create new connection: %w", err)
			}

			// Double-check we haven't exceeded max while creating the connection
			currentCount = c.connections.Len()
			if currentCount >= c.config.MaxConnections {
				c.mu.Unlock()
				newConn.DB.Close()
				log.Printf("Maximum connections reached while creating new connection, closing %s", newConn.ID)
				// Fall through to waiting logic
			} else {
				newConn.InUse = true
				newConn.LastUsed = time.Now()
				c.connections.Set(newConn.ID, newConn)
				actualCount := c.connections.Len()
				c.mu.Unlock()

				log.Printf("New connection %s created on demand (total: %d/%d)", newConn.ID, actualCount, maxConnections)
				return newConn.DB, nil
			}
		} else {
			c.mu.Unlock()
		}
	}

	// If we get here, pool is at max capacity, wait for available connection
	log.Printf("Pool at maximum limit (%d/%d), waiting for available connection...", c.connections.Len(), maxConnections)

	deadline, hasDeadline := ctx.Deadline()
	if hasDeadline && time.Until(deadline) < 50*time.Millisecond {
		return nil, fmt.Errorf("pool full (%d/%d connections) and timeout too short", c.connections.Len(), maxConnections)
	}

	retryTicker := time.NewTicker(50 * time.Millisecond)
	defer retryTicker.Stop()

	for {
		select {
		case <-ctx.Done():
			return nil, fmt.Errorf("context canceled while waiting for connection (pool: %d/%d): %w", c.connections.Len(), maxConnections, ctx.Err())

		case <-retryTicker.C:
			orderedPairs := c.connections.GetOrderedV2()
			for _, pair := range orderedPairs {
				conn := pair.Value
				if !conn.InUse {
					err := c.connections.UpdateWithPointer(pair.Key, func(v **PooledConnection) error {
						if (*v).InUse {
							return errors.New("connection already in use")
						}

						if !c.IsConnectionValid(*v) {
							return errors.New("invalid connection")
						}

						(*v).InUse = true
						(*v).LastUsed = time.Now()
						c.availableCount--
						return nil
					})

					if err == nil {
						log.Printf("Connection %s obtained after waiting", conn.ID)
						return conn.DB, nil
					}
				}
			}
		}
	}
}

func (c *ConnectionInstance) ReleaseConnection(db *sqlx.DB) error {
	if db == nil {
		return errors.New("null connection")
	}

	orderedPairs := c.connections.GetOrderedV2()

	for _, pair := range orderedPairs {
		conn := pair.Value
		if conn.DB == db {
			err := c.connections.UpdateWithPointer(pair.Key, func(v **PooledConnection) error {
				if !(*v).InUse {
					return errors.New("connection was not in use")
				}
				(*v).InUse = false
				(*v).LastUsed = time.Now()
				c.availableCount++
				return nil
			})

			if err != nil {
				return fmt.Errorf("error releasing connection %s: %w", conn.ID, err)
			}

			log.Printf("Connection %s released", conn.ID)
			return nil
		}
	}

	return errors.New("connection not found in pool")
}

func (c *ConnectionInstance) Close() {
	log.Println("Closing connection pool...")

	close(c.stopCh)
	c.wg.Wait()

	orderedPairs := c.connections.GetOrderedV2()
	for _, pair := range orderedPairs {
		conn := pair.Value
		if conn.DB != nil {
			conn.DB.Close()
			log.Printf("Connection %s closed", conn.ID)
		}
	}

	for _, pair := range orderedPairs {
		c.connections.Delete(pair.Key)
	}

	log.Println("Connection pool closed")
}

func (c *ConnectionInstance) GetPoolStats() map[string]interface{} {
	stats := make(map[string]interface{})

	total := c.connections.Len()
	available := 0
	inUse := 0

	orderedPairs := c.connections.GetOrderedV2()
	for _, pair := range orderedPairs {
		if pair.Value.InUse {
			inUse++
		} else {
			available++
		}
	}

	stats["total"] = total
	stats["available"] = available
	stats["in_use"] = inUse
	stats["min_connections"] = c.config.MinConnections
	stats["max_connections"] = c.config.MaxConnections
	stats["uptime_seconds"] = time.Since(c.created).Seconds()

	connections := make([]map[string]interface{}, 0)
	for _, pair := range orderedPairs {
		conn := pair.Value
		connInfo := map[string]interface{}{
			"id":           conn.ID,
			"in_use":       conn.InUse,
			"created_at":   conn.CreatedAt,
			"last_used":    conn.LastUsed,
			"age_seconds":  time.Since(conn.CreatedAt).Seconds(),
			"idle_seconds": time.Since(conn.LastUsed).Seconds(),
		}
		connections = append(connections, connInfo)
	}
	stats["connections"] = connections

	return stats
}

func (c *ConnectionInstance) TriggerExpireConnections() {
	select {
	case c.expireConnectionsCh <- true:
		log.Println("Expire connections requested")
	default:
		log.Println("Expire request already in progress")
	}
}

func (c *ConnectionInstance) TriggerRenewConnections() {
	select {
	case c.renewConnectionsCh <- true:
		log.Println("Renew connections requested")
	default:
		log.Println("Renew request already in progress")
	}
}

func (c *ConnectionInstance) ResetPool() error {
	log.Println("Resetting connection pool...")

	orderedPairs := c.connections.GetOrderedV2()
	for _, pair := range orderedPairs {
		conn := pair.Value
		if conn.DB != nil {
			conn.DB.Close()
		}
		c.connections.Delete(pair.Key)
	}

	for i := 0; i < c.config.MinConnections; i++ {
		conn, err := c.createConnection()
		if err != nil {
			log.Printf("Error recreating connection %d: %v", i, err)
			continue
		}
		c.connections.Set(conn.ID, conn)
	}

	c.availableCount = int32(c.config.MinConnections)
	log.Printf("Pool reset with %d connections", c.connections.Len())
	return nil
}