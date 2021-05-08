package logs

const (
	DEBUG int8 = iota - 1
	INFO
	WARN
	ERROR
	FATAL
)

func Init() {
}

func Flush() {
}
