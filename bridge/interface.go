package bridge

type VideoPublisher interface {
	Write(data []byte) error
	Close()
}
