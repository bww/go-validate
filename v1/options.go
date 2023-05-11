package validate

type Config struct {
	CheckTag string
	ErrorTag string
	FieldTag string
	BasePath string
}

func (c Config) WithOptions(opts []Option) Config {
	for _, opt := range opts {
		c = opt(c)
	}
	return c
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

func BasePath(path string) Option {
	return func(c Config) Config {
		c.BasePath = path
		return c
	}
}
