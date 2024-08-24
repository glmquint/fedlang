## Fedlang

- Source code used for the manuscript “Devising an Actor-based Middleware Support to Federated Learning Experiments and Systems”, currently under review.
- This work has been developed by the Artificial Intelligence R&D Group at the Department of Information Engineering, University of Pisa.
- Authors:
- - Alessio Bechini ([scholar](https://scholar.google.com/citations?user=ooYOGP4AAAAJ)) (alessio.bechini@unipi.it)
  - José Luis Corcuera Bárcena ([scholar](https://scholar.google.it/citations?user=dasDbcAAAAAJ)) (joseluis.corcuera@phd.unipi.it)

### Installation
Clone this repository
```bash
git clone https://github.com/Pyrlang/Pyrlang.git && git clone https://github.com/Pyrlang/Term.git
```
```bash
mkdir stats logs
```

### Usage

#### create docker network
```bash
docker network rm fednet; docker network create --subnet 172.19.0.0/16 fednet
```

#### launch distributed nodes
```bash
CGO_ENABLED=0 go build -C src/go/client/ -o fcmeans_client && CGO_ENABLED=0 go build -C src/go/server/ -o fcmeans_server && ./start_distributed.sh <LANGUAGE: go/python> <NUM_CLIENTS>
```
