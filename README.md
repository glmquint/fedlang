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

### Usage (Docker)

#### create docker network
```bash
docker network rm fednet; docker network create --subnet 172.19.0.0/16 fednet
```

#### launch distributed nodes
```bash
CGO_ENABLED=0 go build -C src/go/client/ -o fcmeans_client && CGO_ENABLED=0 go build -C src/go/server/ -o fcmeans_server && ./start_distributed.sh 3 go
```

### Usage (Distributed)

#### On the director 
```bash
pipenv run sh run_director.sh
```

#### On clients
```bash
FL_CLIENT_ID=<client_id> FL_DIRECTOR_IP=<director_ip> pipenv run sh start_client.sh
```

#### On stats node
```bash
FL_DIRECTOR_NAME=director@<director_ip> FL_NUMBER_CLIENTS=<number of clients> FL_LANGUAGE=<go|python> ./start_stats_node.sh
```