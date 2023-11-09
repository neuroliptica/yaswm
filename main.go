package main

func main() {
	go defaultReader.Read()
	defer defaultReader.WaitFinish()

	logger := MakeLogger("main").BindChanReader(&defaultReader)
	logger.Log("hello world")
	logger.Log("sieg heil")
}
