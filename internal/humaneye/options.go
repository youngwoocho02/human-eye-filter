package humaneye

type Options struct {
	MaxLines      int
	MaxChars      int
	MaxInputBytes int64
	AutoMinLines  int
	AutoMinChars  int
	Mode          string
	Tool          string
	Keep          string
	Focus         string
	RawOnFail     bool
}

func DefaultOptions() Options {
	return Options{
		MaxLines:      160,
		MaxChars:      12000,
		MaxInputBytes: 4 * 1024 * 1024,
		AutoMinLines:  40,
		AutoMinChars:  4000,
		Mode:          "auto",
		Tool:          "",
		Keep:          "error",
		Focus:         "",
		RawOnFail:     true,
	}
}
