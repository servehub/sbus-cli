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

### Usage

Without --env parameter, default environment is local. (--env=local)
```shell script
sbus orders.create-order '{"price":"3.141592"}'
```

--env: Send request to a specific environment.
```shell script
sbus orders.create-order '{"price":"3.141592"}' --env=qa
```

--file: Indicates that request body (2. parameter), is a file and payload should be read from this file.
```shell script
sbus orders.create-order '/home/my-payloads/createorder.json' --file --env=qa
```

--no-status: Status code (like 200, 400) is omitted in response. Useful if you are parsing response with jq.
```shell script
sbus orders.create-order '{"price":"3.141592"}' --no-status --env=qa
```

--event: Send an event
```shell script
sbus orders.order-updated '{"orderId":"123"}' --env=qa --event 
```

```shell script
sbus register users/joe.smith --save-to-consul="consul.qa.example.co" --group=devs --group=support --group=leads --env=qa
```

```shell
sbus --help
sbus send --help
```
