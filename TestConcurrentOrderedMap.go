package concurrentmap

func TestConcurrentAccess(t *testing.T) {
    m := NewConcurrentOrderedMap[int, string]()
    var wg sync.WaitGroup
    
    for i := 0; i < 100; i++ {
        wg.Add(1)
        go func(id int) {
            defer wg.Done()
            m.Set(id, fmt.Sprintf("value-%d", id))
        }(i)
    }
    
    wg.Wait()
}
