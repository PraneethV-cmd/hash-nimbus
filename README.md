# Hash Nimbus

**Hash Nimbus** is a simple distributed key-value store that uses the **Raft consensus model** for leader election, log replication, and persistence. It ensures strong consistency and fault tolerance across multiple nodes.

## Features
- **Leader Election**: Uses the Raft algorithm to elect a leader.
- **Log Replication**: Ensures data consistency across nodes.
- **Persistence**: Stores logs for recovery after crashes.
- **HTTP API**: Simple endpoints for storing and retrieving key-value pairs.

## Installation
Ensure you have Go installed. Then, build the project:

```sh
cd cmd/kvapi
go build
```

## Running the Cluster
Open three terminals and start three nodes:

```sh
./kvapi --node 0 --http :2021 --cluster "1,:3031;2,:3032;3,:3033"
./kvapi --node 1 --http :2022 --cluster "1,:3031;2,:3032;3,:3033"
./kvapi --node 2 --http :2023 --cluster "1,:3031;2,:3032;3,:3033"
```

## Using the Key-Value Store

### Setting a Key-Value Pair
Send a `SET` request to store data in the cluster:

```sh
curl "http://localhost:2021/set?key=test&value=success"
```

### Retrieving a Value
You can retrieve the value from any node:

```sh
curl "http://localhost:2022/get?key=test"
```

## Architecture
Hash Nimbus consists of multiple nodes, one of which is elected as the leader. The leader handles all write operations, while followers replicate logs. If the leader fails, a new leader is elected.

### Flow of Operations:
1. A client sends a `SET` request to any node.
2. If the node is a follower, it forwards the request to the leader.
3. The leader logs the operation and replicates it to followers.
4. Once a majority acknowledges, the leader commits the change and responds.
5. Any node can handle `GET` requests since the data is consistent across the cluster.

## Future Improvements
- **Snapshotting**: To prevent log growth.
- **Automatic Rebalancing**: Handle node joins and exits dynamically.
- **Optimized Storage**: Use persistent databases like BoltDB or BadgerDB.

## Contributing
Feel free to open issues or submit pull requests!

## License
MIT License


