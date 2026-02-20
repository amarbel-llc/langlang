package main

func f() {
	ch := make(chan int)
	done := make(chan struct{})

	go func() {
		ch <- 42
		close(done)
	}()

	v := <-ch
	_ = v

	<-done

	// buffered channel
	buf := make(chan string, 10)
	buf <- "hello"
	msg := <-buf
	_ = msg
}

func selectExample(ch1, ch2 chan int, quit chan struct{}) {
	for {
		select {
		case v := <-ch1:
			_ = v
		case v := <-ch2:
			_ = v
		case ch1 <- 0:
		case <-quit:
			return
		default:
		}
	}
}
