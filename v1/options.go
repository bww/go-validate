package validate

type Config struct {
	CheckTag string
	ErrorTag string
	FieldTag string
}

type Option func(Config) Config

func Mode(name string) Option {
	return CheckTag(name)
}

func CheckTag(name string) Option {
	return func(c Config) Config {
		c.CheckTag = name
		return c
	}
}

func ErrorTag(name string) Option {
	return func(c Config) Config {
		c.ErrorTag = name
		return c
	}
}

func FieldTag(name string) Option {
	return func(c Config) Config {
		c.FieldTag = name
		return c
	}
}
