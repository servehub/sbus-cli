# sbus-go

```shell script
sbus orders.create-order '{"price":"3.141592"}'
```

```shell script
sbus orders.create-order '{"price":"3.141592"}' --env=qa
```


Configure access to env specific rabbitmq:

```shell script
export SBUS_AMQP_QA_URL="amqp://guest:guest@rabbit.qa.example.com:5672/"
export SBUS_AMQP_STAGE_URL="amqp://guest:guest@rabbit.stage.example.com:5672/"
export SBUS_AMQP_LIVE_URL="amqp://guest:guest@rabbit.live.example.com:5672/"
```
