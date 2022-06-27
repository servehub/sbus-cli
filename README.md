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

export SBUS_USER=users/joe.bloggs  

export SBUS_QA_PRIVATE_KEY=... 
export SBUS_STAGE_PRIVATE_KEY=... 
export SBUS_LIVE_PRIVATE_KEY=... 

export SBUS_CONSUL_QA_URL="https://consul.qa.example.com" 
export SBUS_CONSUL_STAGE_URL="https://consul.stage.example.com" 
export SBUS_CONSUL_LIVE_URL="https://consul.live.example.com" 

export SBUS_CONSUL_QA_DC="qa1" 
export SBUS_CONSUL_STAGE_DC="stage1" 
export SBUS_CONSUL_LIVE_DC="live1" 
```

### Usage

```shell script
sbus send orders.create-order '{"price":"3.141592"}'
sbus orders.create-order '{"price":"3.141592"}'
```

```shell script
sbus --env=qa send orders.create-order '{"price":"3.141592"}'
sbus --env=qa orders.create-order '{"price":"3.141592"}'
```

```shell script
sbus --env=qa send --event orders.order-updated '{"orderId":"123"}'
sbus --env=qa --event orders.order-updated '{"orderId":"123"}'
```

```shell script
sbus --env=qa register --save-consul --group=devs --group=support users/joe.bloggs
```

```shell
sbus --env=qa verify WC_udKNSxhBqS7hcYE89RlIKN0RqZejK1QZrzviSlgCD0ijlz8MK4w8gtQe4XGEmWVfNECOwF1xLFMsNrYWPCw== my/service '{}' 1656322991130
```

```shell
sbus --help
```
