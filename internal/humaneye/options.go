package humaneye

type Options struct {
	MaxLines      int
	MaxChars      int
	MaxInputBytes int64
	Focus         string
	RawOnFail     bool
}

func DefaultOptions() Options {
	return Options{
		MaxLines:      160,
		MaxChars:      12000,
		MaxInputBytes: 4 * 1024 * 1024,
		Focus:         "",
		RawOnFail:     true,
	}
}
