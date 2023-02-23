package config

import (
	"fmt"
	"os"
	"os/user"
	"strings"

	"github.com/spf13/viper"
)

type EnvData struct {
	SbusUser       string `mapstructure:"sbus_user"`
	SbusPrivateKey string `mapstructure:"sbus_private_key"`
	SbusAmqpUrl    string `mapstructure:"sbus_amqp_url"`
	SbusKafkaUrl   string `mapstructure:"sbus_kafka_url"`
}

type AppConfig struct {
	Env      map[string]EnvData `mapstructure:"env"`
	SbusUser string             `mapstructure:"sbus_user"`
}
type EnvVariable int64

const (
	EnvSbusUser EnvVariable = iota
	EnvSbusPrivateKey
	EnvSbusAmqpUrl
	EnvKafkaUrl
)

/* Get value for environment from config file or from ENV variable if exist */
func (ac *AppConfig) GetValue(env *string, envVariable EnvVariable) (string, bool) {
	var configValue string
	envValue := fixDashInEnvVariable(env)

	switch envVariable {
	case EnvSbusUser:
		// if user for specific env don't exist try global user
		configValue = ac.Env[envValue].SbusUser
		if configValue == "" {
			configValue = ac.SbusUser
		}
	case EnvSbusPrivateKey:
		configValue = ac.Env[envValue].SbusPrivateKey
	case EnvSbusAmqpUrl:
		configValue = ac.Env[envValue].SbusAmqpUrl
	case EnvKafkaUrl:
		configValue = ac.Env[envValue].SbusKafkaUrl
	}

	if configValue == "" {
		return "", false
	}
	return configValue, true
}

/* Sat AMPQ url value for specific environment */
func (ac *AppConfig) SetValueAmpqUrl(env *string, ampqUrl *string) {
	envValue := fixDashInEnvVariable(env)
	if ac.Env == nil {
		ac.Env = make(map[string]EnvData)
	}
	envData := ac.Env[envValue]
	envData = EnvData{SbusAmqpUrl: *ampqUrl, SbusUser: envData.SbusUser, SbusPrivateKey: envData.SbusPrivateKey, SbusKafkaUrl: envData.SbusKafkaUrl}
	ac.Env[envValue] = envData
}

/* Sat Kafka url value for specific environment */
func (ac *AppConfig) SetValueKafkaUrl(env *string, kafkaUrl *string) {
	envValue := fixDashInEnvVariable(env)
	if ac.Env == nil {
		ac.Env = make(map[string]EnvData)
	}
	envData := ac.Env[envValue]
	envData = EnvData{SbusAmqpUrl: envData.SbusAmqpUrl, SbusUser: envData.SbusUser, SbusPrivateKey: envData.SbusPrivateKey, SbusKafkaUrl: *kafkaUrl}
	ac.Env[envValue] = envData
}

/* Load configuration from config.yml file, don't override any value from ENV variables */
func LoadConfigNoOverrides(targetEnv *string) (*AppConfig, error) {
	envValue := fixDashInEnvVariable(targetEnv)
	return loadConfig(&envValue, false)
}

/*
	Load configuration from config.yml file, override value from ENV variables.

Overrides available:
  - <env>.sbus_user override with ENV SBUS_USER
  - <env>.sbus_private_key override with ENV SBUS_<env>_PRIVATE_KEY
  - <env>.sbus_amqp_url override with ENV SBUS_AMQP_<env>_URL
*/
func LoadConfigWithOverrides(envName *string) (*AppConfig, error) {
	envValue := fixDashInEnvVariable(envName)
	return loadConfig(&envValue, true)
}

/* Saving actual configuration to file, if file doesn't exist it be will created */
func (ac *AppConfig) SaveConfiguration() {
	viper.Set("env", ac.Env)

	usr, err := user.Current()
	_, err = os.Stat(fmt.Sprintf("%s/.sbus", usr.HomeDir))
	if os.IsNotExist(err) {
		err := os.Mkdir(fmt.Sprintf("%s/.sbus", usr.HomeDir), 0755)
		if err != nil {
			fmt.Println("For some reason I can't create folder for configuration. New configuraiton will not be created, please create manually %HOME/.sbus folder", err)
		}
	}

	err = viper.WriteConfig()
	if err != nil {
		err = viper.SafeWriteConfig()
		if err != nil {
			return
		}
	}
	fmt.Println("New configuration saved.")

}

func loadConfig(targetEnv *string, overrideConfigWithEnv bool) (*AppConfig, error) {
	viper.AddConfigPath("$HOME/.sbus")
	viper.SetConfigName("config")
	viper.SetConfigType("yml")

	if overrideConfigWithEnv {
		viper.AutomaticEnv()
		setEnvOverrides(targetEnv)
	}

	_ = viper.ReadInConfig()

	config := AppConfig{}
	err := viper.Unmarshal(&config)
	return &config, err
}

func setEnvOverrides(targetEnv *string) {
	envValue := fixDashInEnvVariable(targetEnv)
	if targetEnv != nil {
		_ = viper.BindEnv("sbus_user", "SBUS_USER")

		configKey := fmt.Sprintf("env.%s.sbus_user", envValue)
		_ = viper.BindEnv(configKey, "SBUS_USER")

		configKey = fmt.Sprintf("env.%s.sbus_amqp_url", envValue)
		envKey := fmt.Sprintf("SBUS_AMQP_%s_URL", strings.ToUpper(envValue))
		_ = viper.BindEnv(configKey, envKey)

		configKey = fmt.Sprintf("env.%s.sbus_kafka_url", envValue)
		envKey = fmt.Sprintf("SBUS_KAFKA_%s_URL", strings.ToUpper(envValue))
		_ = viper.BindEnv(configKey, envKey)

		configKey = fmt.Sprintf("env.%s.sbus_private_key", envValue)
		envKey = fmt.Sprintf("SBUS_%s_PRIVATE_KEY", strings.ToUpper(envValue))
		_ = viper.BindEnv(configKey, envKey)
	}
}

func fixDashInEnvVariable(envVariable *string) string {
	fixDashInEnv := strings.ReplaceAll(*envVariable, "-", "_")
	return fixDashInEnv
}
