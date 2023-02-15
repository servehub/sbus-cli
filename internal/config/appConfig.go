package config

import (
	"fmt"
	"os"
	"os/user"
	"strings"

	"github.com/spf13/viper"
)

type EnvData struct {
	Sbus_User        string
	Sbus_Private_Key string
	Sbus_Amqp_Url    string
}

type AppConfig struct {
	Env       map[string]EnvData
	Sbus_User string
}
type EnvVariable int64

const (
	EnvSbusUser EnvVariable = iota
	EnvSbusPrivateKey
	EnvSbusAmqpUrl
)

/* Get value for environment from config file or from ENV variable if exist */
func (ac *AppConfig) GetValue(env *string, envVariable EnvVariable) (string, bool) {
	var configValue string
	fixDashInEnv := strings.ReplaceAll(*env, "-", "_")

	switch envVariable {
	case EnvSbusUser:
		// if user for specific env don't exist try global user
		configValue = ac.Env[fixDashInEnv].Sbus_User
		if configValue == "" {
			configValue = ac.Sbus_User
		}
	case EnvSbusPrivateKey:
		configValue = ac.Env[fixDashInEnv].Sbus_Private_Key
	case EnvSbusAmqpUrl:
		configValue = ac.Env[fixDashInEnv].Sbus_Amqp_Url
	}

	if configValue == "" {
		return "", false
	}
	return configValue, true
}

/* Sat AMPQ url value for specific environment */
func (ac *AppConfig) SetValueAmpqUrl(env *string, ampqUrl *string) {
	fixDashInEnv := strings.ReplaceAll(*env, "-", "_")
	if ac.Env == nil {
		ac.Env = make(map[string]EnvData)
	}
	envData := ac.Env[fixDashInEnv]
	envData = EnvData{Sbus_Amqp_Url: *ampqUrl, Sbus_User: envData.Sbus_User, Sbus_Private_Key: envData.Sbus_Private_Key}
	ac.Env[fixDashInEnv] = envData
}

/* Load configuration from config.yml file, don't override any value from ENV variables */
func LoadConfigNoOverrides(targetEnv *string) (*AppConfig, error) {
	fixDashInEnv := strings.ReplaceAll(*targetEnv, "-", "_")
	return loadConfig(&fixDashInEnv, false)
}

/*
	Load configuration from config.yml file, override value from ENV variables.

Overrides available:
  - <env>.sbus_user override with ENV SBUS_USER
  - <env>.sbus_private_key override with ENV SBUS_<env>_PRIVATE_KEY
  - <env>.sbus_amqp_url override with ENV SBUS_AMQP_<env>_URL
*/
func LoadConfigWithOverrides(envName *string) (*AppConfig, error) {
	fixDashInEnv := strings.ReplaceAll(*envName, "-", "_")
	return loadConfig(&fixDashInEnv, true)
}

/* Saving actual configuration to file, if file doesn't exist it be will created */
func (ac *AppConfig) SaveConfiguration() {
	viper.Set("env", ac.Env)

	usr, err := user.Current()
	_, err = os.Stat(fmt.Sprintf("%s/.sbus", usr.HomeDir))
	if os.IsNotExist(err) {
		err := os.Mkdir(fmt.Sprintf("%s/.sbus", usr.HomeDir), 0755)
		if err != nil {
			fmt.Println("For some reaseone I can't create folder for configuration. New configuraiton will not be created, please create manually %HOME/.sbus folder", err)
		}
	}

	err = viper.WriteConfig()
	if err != nil {
		err = viper.SafeWriteConfig()
		if err != nil {
			return
		}
	}
	fmt.Println("New configration saved.")

}

func loadConfig(targetEnv *string, overrideConfigWithEnv bool) (*AppConfig, error) {
	viper.AddConfigPath("$HOME/.sbus")
	viper.SetConfigName("config")
	viper.SetConfigType("yml")

	viper.AutomaticEnv()
	setEnvOverrides(targetEnv)

	_ = viper.ReadInConfig()

	config := AppConfig{}
	err := viper.Unmarshal(&config)
	return &config, err
}

func setEnvOverrides(targetEnv *string) {
	fixDashInEnv := strings.ReplaceAll(*targetEnv, "-", "_")
	if targetEnv != nil {
		_ = viper.BindEnv("sbus_user", "SBUS_USER")

		configKey := fmt.Sprintf("env.%s.sbus_user", fixDashInEnv)
		_ = viper.BindEnv(configKey, "SBUS_USER")

		configKey = fmt.Sprintf("env.%s.sbus_amqp_url", fixDashInEnv)
		envKey := fmt.Sprintf("SBUS_AMQP_%s_URL", strings.ToUpper(fixDashInEnv))
		_ = viper.BindEnv(configKey, envKey)

		configKey = fmt.Sprintf("env.%s.sbus_private_key", fixDashInEnv)
		envKey = fmt.Sprintf("SBUS_%s_PRIVATE_KEY", strings.ToUpper(fixDashInEnv))
		_ = viper.BindEnv(configKey, envKey)
	}
}
