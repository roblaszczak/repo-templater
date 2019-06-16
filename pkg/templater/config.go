package templater

type RepositoryConfig struct {
	Name      string
	HumanName string
	URL       string
	Templates []string

	Variables map[string]interface{}
}

type Config struct {
	Repositories []RepositoryConfig
}
