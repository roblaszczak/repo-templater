package templater

type RepositoryConfig struct {
	Name      string
	HumanName string
	URL       string
	Templates []string

	Variables map[string]string

	RunCmds [][]string
}

func makeTemplateVariables(repoConfig RepositoryConfig, config Config) map[string]interface{} {
	vars := map[string]interface{}{}

	for key, value := range repoConfig.Variables {
		vars[key] = value
	}

	vars["Name"] = repoConfig.Name
	vars["HumanName"] = repoConfig.HumanName
	vars["URL"] = repoConfig.URL
	vars["Config"] = config

	return vars
}

type Config struct {
	Repositories []*RepositoryConfig

	CommonVariables map[string]string
}
