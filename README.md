# Vertex Relay

**Vertex Relay** is the backbone endpoint powering the full suite of Vertex services.  
It is built on the lightweight and efficient [rely](https://github.com/pippellia-btc/rely) framework.  

## Features

- **DVM Execution**: Seamlessly runs Vertex Data Vending Machines (DVMs) with low latency, ensuring a smooth experience for users.  

- **Profile Discovery**: Stores profile events like `kind:0`, `kind:3`, `kind:10000` ..., making it easy to discover and verify anyone.

- **Read-Only**: To maintain a spam-free environment, the relay does not accept direct user writes. Only our optimized [crawler](https://github.com/vertex-lab/crawler_v2) writes to the database, guaranteeing the highest-quality, most up-to-date events.

## Vertex Services

Learn more about the Vertex ecosystem and its services in our [docs](https://vertexlab.io/docs/nips/).  

## License

Licensed under MIT. For full details, see the [license](https://github.com/vertex-lab/relay/blob/master/LICENSE.md).  

