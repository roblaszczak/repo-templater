package templater

type RepositoryConfig struct {
	Name      string
	HumanName string
	URL       string
	Templates []string

	Variables map[string]string

	RunCmds [][]string
}

type Config struct {
	Repositories []*RepositoryConfig

	CommonVariables map[string]string
}
