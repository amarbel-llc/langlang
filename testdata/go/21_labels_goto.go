package main

func withLabel() {
outer:
	for i := 0; i < 10; i++ {
		for j := 0; j < 10; j++ {
			if i+j == 15 {
				break outer
			}
		}
	}
}

func withContinueLabel() {
next:
	for i := 0; i < 10; i++ {
		for j := 0; j < 10; j++ {
			if j%2 == 0 {
				continue next
			}
		}
	}
}

func withGoto() {
	i := 0
loop:
	if i >= 10 {
		goto done
	}
	i++
	goto loop
done:
	_ = i
}
