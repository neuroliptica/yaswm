package main

import (
	"fmt"
	"log"
	"sync"
)

var defaultLogWg sync.WaitGroup

var defaultReader = ChanReader{
	Wg:   &defaultLogWg,
	Chan: make(chan string),
}

type ChanReader struct {
	Chan chan string
	Wg   *sync.WaitGroup
}

func (ch *ChanReader) Write(msg string) {
	ch.Wg.Add(1)
	ch.Chan <- msg
}

// Should be runned in separate goroutine.
func (ch *ChanReader) Read() {
	for msg := range ch.Chan {
		log.Println(msg)
		ch.Wg.Done()
	}
}

// For correct logs output when terminating.
func (ch *ChanReader) WaitFinish() {
	ch.Wg.Wait()
}

type Logger struct {
	Tag  string
	Chan *ChanReader
}

func MakeLogger(tag string) *Logger {
	return &Logger{
		Tag: tag,
	}
}

func (l *Logger) BindChanReader(ch *ChanReader) *Logger {
	l.Chan = ch
	return l
}

func (l *Logger) Log(msg ...any) {
	l.Chan.Write(fmt.Sprint(msg...))
}

func (l *Logger) Logf(format string, entries ...any) {
	l.Chan.Write(fmt.Sprintf(format, entries...))
}
