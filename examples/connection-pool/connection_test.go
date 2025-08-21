package test

import (
	"context"
	//"YOUR_PROJECT/model"
	"sync"
	"testing"
	"time"

	"github.com/jmoiron/sqlx"
)

func TestConnectionSingleton(t *testing.T) {
	instance1 := model.GetConnectionInstance()
	instance2 := model.GetConnectionInstance()
	if instance1 != instance2 {
		t.Error("GetConnectionInstance is not returning the same instance (singleton failed)")
	}
}

func TestConnectionConcurrentSingleton(t *testing.T) {
	instances := make([]*model.ConnectionInstance, 100)
	var wg sync.WaitGroup
	for i := 0; i < 100; i++ {
		wg.Add(1)
		go func(index int) {
			defer wg.Done()
			instances[index] = model.GetConnectionInstance()
		}(i)
	}
	wg.Wait()
	first := instances[0]
	for i, inst := range instances {
		if inst != first {
			t.Errorf("Instance %d is different from the first (singleton failed under concurrency)", i)
		}
	}
}

func TestInstanceConnectionCreation(t *testing.T) {
	pool := model.GetConnectionInstance()
	stats := pool.GetPoolStats()
	total, ok := stats["total"].(int)
	if !ok {
		t.Fatal("Could not get total number of connections")
	}
	minConnections, ok := stats["min_connections"].(int)
	if !ok {
		t.Fatal("Could not get min_connections")
	}
	if total < minConnections {
		t.Errorf("Pool created with %d connections, expected minimum of %d", total, minConnections)
	}
}

func TestConnectionGetAndReleaseConnection(t *testing.T) {
	pool := model.GetConnectionInstance()
	ctx := context.Background()
	db, err := pool.GetConnection(ctx)
	if err != nil {
		t.Fatalf("Error getting connection: %v", err)
	}
	if db == nil {
		t.Fatal("Returned connection is nil")
	}
	stats := pool.GetPoolStats()
	inUse, _ := stats["in_use"].(int)
	if inUse < 1 {
		t.Error("in_use statistic was not updated after getting connection")
	}
	err = pool.ReleaseConnection(db)
	if err != nil {
		t.Fatalf("Error releasing connection: %v", err)
	}
	stats = pool.GetPoolStats()
	inUse, _ = stats["in_use"].(int)
	available, _ := stats["available"].(int)
	if available < 1 {
		t.Error("Connection was not marked as available after release")
	}
}

func TestConnectionConcurrentAccess(t *testing.T) {
	pool := model.GetConnectionInstance()
	var wg sync.WaitGroup
	errors := make(chan error, 50)
	for i := 0; i < 50; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
			defer cancel()
			db, err := pool.GetConnection(ctx)
			if err != nil {
				errors <- err
				return
			}
			time.Sleep(10 * time.Millisecond)
			var result int
			err = db.QueryRow("SELECT 1").Scan(&result)
			if err != nil {
				errors <- err
			}
			if err := pool.ReleaseConnection(db); err != nil {
				errors <- err
			}
		}(i)
	}
	wg.Wait()
	close(errors)
	errorCount := 0
	for err := range errors {
		t.Logf("Error in concurrent access: %v", err)
		errorCount++
	}
	if errorCount > 5 {
		t.Errorf("Too many errors in concurrent access: %d", errorCount)
	}
}

func TestInstanceConnectionExpiration(t *testing.T) {
	pool := model.GetConnectionInstance()
	statsInitial := pool.GetPoolStats()
	totalInitial, _ := statsInitial["total"].(int)
	pool.TriggerExpireConnections()
	time.Sleep(2 * time.Second)
	statsFinal := pool.GetPoolStats()
	totalFinal, _ := statsFinal["total"].(int)
	minConnections, _ := statsFinal["min_connections"].(int)
	if totalFinal < minConnections {
		t.Errorf("Pool has %d connections after expiration, expected minimum of %d", totalFinal, minConnections)
	}
	t.Logf("Connections before: %d, after: %d", totalInitial, totalFinal)
}

func TestInstanceMaxConnections(t *testing.T) {
	pool := model.GetConnectionInstance()
	stats := pool.GetPoolStats()
	maxConnections, _ := stats["max_connections"].(int)
	connections := make([]*sqlx.DB, 0)
	ctx := context.Background()
	for i := 0; i < maxConnections+5; i++ {
		ctxTimeout, cancel := context.WithTimeout(ctx, 100*time.Millisecond)
		db, err := pool.GetConnection(ctxTimeout)
		cancel()
		if err == nil {
			connections = append(connections, db)
		}
	}
	if len(connections) > maxConnections {
		t.Errorf("Pool allowed %d connections, configured maximum is %d", len(connections), maxConnections)
	}
	for _, db := range connections {
		pool.ReleaseConnection(db)
	}
	t.Logf("Obtained %d connections out of a maximum of %d", len(connections), maxConnections)
}

func TestInstancePoolReset(t *testing.T) {
	pool := model.GetConnectionInstance()
	ctx := context.Background()
	db, err := pool.GetConnection(ctx)
	if err != nil {
		t.Fatalf("Error getting connection before reset: %v", err)
	}
	err = pool.ResetPool()
	if err != nil {
		t.Fatalf("Error resetting pool: %v", err)
	}
	err = pool.ReleaseConnection(db)
	if err == nil {
		t.Error("Expected error when releasing old connection after reset")
	}
	db2, err := pool.GetConnection(ctx)
	if err != nil {
		t.Fatalf("Error getting connection after reset: %v", err)
	}
	if db2 == nil {
		t.Fatal("New connection is nil after reset")
	}
	err = pool.ReleaseConnection(db2)
	if err != nil {
		t.Errorf("Error releasing new connection: %v", err)
	}
}

func TestInstanceConnectionValidation(t *testing.T) {
	pool := model.GetConnectionInstance()
	conn := &model.PooledConnection{
		DB:        nil,
		CreatedAt: time.Now().Add(-1 * time.Hour),
		LastUsed:  time.Now().Add(-30 * time.Minute),
		ID:        "test_invalid",
		InUse:     false,
	}
	if pool.IsConnectionValid(conn) {
		t.Error("Invalid connection was considered valid")
	}
	ctx := context.Background()
	db, err := pool.GetConnection(ctx)
	if err != nil {
		t.Fatalf("Error getting connection: %v", err)
	}
	defer pool.ReleaseConnection(db)
}

func BenchmarkInstanceGetConnection(b *testing.B) {
	pool := model.GetConnectionInstance()
	ctx := context.Background()
	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			db, err := pool.GetConnection(ctx)
			if err != nil {
				b.Fatalf("Error getting connection: %v", err)
			}
			pool.ReleaseConnection(db)
		}
	})
}

func BenchmarkInstanceConcurrentOrderedMap(b *testing.B) {
	pool := model.GetConnectionInstance()
	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			stats := pool.GetPoolStats()
			_ = stats["total"]
		}
	})
}
