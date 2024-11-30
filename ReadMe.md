# CBDC Network
This is a demo CBDC network.

## SetUp
```make protobuf```

### Start Network
```cd ./network```

```./network.sh up createChannel -c retail```

```./network.sh deployCC -ccn cbdc -ccp ../token-erc-20/chaincode-go/ -ccl go -c retail```