# sbus-cli


### Install

```
brew install servehub/tap/sbus-cli
```

### Configure

Configure access to env specific rabbitmq:

```shell script
export SBUS_AMQP_QA_URL="amqp://guest:guest@rabbit.qa.example.com:5672/"
export SBUS_AMQP_STAGE_URL="amqp://guest:guest@rabbit.stage.example.com:5672/"
export SBUS_AMQP_LIVE_URL="amqp://guest:guest@rabbit.live.example.com:5672/"
```

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
