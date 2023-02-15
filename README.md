# sbus-cli


### Install

```
brew install servehub/tap/sbus-cli
```

### Configure

Configure access to env specific rabbitmq, identity/private key, and optionally consul (and datacenter if not dc1):

```shell script
export SBUS_AMQP_QA_URL="amqp://guest:guest@rabbit.qa.example.com:5672/"
export SBUS_AMQP_STAGE_URL="amqp://guest:guest@rabbit.stage.example.com:5672/"
export SBUS_AMQP_LIVE_URL="amqp://guest:guest@rabbit.live.example.com:5672/"

export SBUS_USER='users/joe.smith'
export SBUS_QA_PUBLIC_KEY=db41b9d8d231f***88f5fa007ce5 
export SBUS_QA_PRIVATE_KEY=daf163359fb9***8863642af8029f5fa007ce5
```

Or use config file in $HOME/.sbus/config.yml

Example of config file.
```
env:
    qa:
        sbus_user: "users/joe.smith"
        sbus_private_key: "daf163359fb9***8863642af8029f5fa007ce5"
        sbus_amqp_url: "amqp://guest:guest@rabbit.qa.example.com:5672/"
    stage:
        sbus_user: "users/stage.doe"
        sbus_private_key: "ccc163359fb9***8863642af8029f5fa007ce5"
        sbus_amqp_url: "amqp://guest:guest@rabbit.stage.example.com:5672/"
```
Values from config file can be overriden by ENV variabls. For example:
If you have variable
```
SBUS_AMQP_STAGE_URL="amqp://guest:guest@rabbit.stage.example.com:5672/"
```
it will override env.qa.sbus_amqp_url value from config file, same principle works for other variables in config file. Except for SBUS_USER, that env variable will override all sbus_user value for all environemnt in config file. 

### Load AMQP_URL from AWS Secret Manager

By running *config* command we will connect to AWS Secret Manager and fetch amqp_url for some environemnt and save it to config.yml
There is two way:
  - we know name of secret in AWS Secret Manager then we run
  ```
  sbus config  --sm-key=secret_name_1  --env=test
   ```
This command will connect to AWS SM , get secret under name secret_name_1 and save/add it to config.yml under test env

  - we don't know secret name but we know environement name
  ```
  sbus config  --env=test
   ```
This command will connect to AWS SM, it will search for secret with name containing "test" and ends with amqp_url. And if only one secret exist that match criteria it will be fetched and save in config.yml under config.yml file.

### Usage

```shell script
sbus orders.create-order '{"price":"3.141592"}'
```

```shell script
sbus orders.create-order '{"price":"3.141592"}' --env=qa
```

```shell script
sbus orders.order-updated '{"orderId":"123"}' --env=qa --event 
```

```shell script
sbus register users/joe.smith --save-to-consul="consul.qa.example.co" --group=devs --group=support --group=leads --env=qa
```

```shell
sbus --help
```
