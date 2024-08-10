FROM erlang:25.3.2.6-slim

WORKDIR /app

RUN apt-get update
RUN apt-get -y install unzip zip procps build-essential python3-pip net-tools curl git tmux
COPY . /app/fedlang
RUN curl https://sh.rustup.rs -sSf | bash -s -- -y
RUN . "$HOME/.cargo/env"
RUN pip3 install --upgrade pip 
RUN pip install --upgrade pip 
RUN pip3 install pyopenssl setuptools-rust semantic_version pipenv
WORKDIR /app/fedlang

RUN pipenv -v install
RUN pipenv -v install pandas numba

ENV PATH="/root/.cargo/bin:${PATH}"

WORKDIR /app/fedlang/Term
RUN pipenv -v install --deploy --ignore-pipfile
RUN pipenv run pip install .

WORKDIR /app/fedlang/Pyrlang
RUN pipenv -v install --deploy --ignore-pipfile
RUN pipenv run pip install .

WORKDIR /app/fedlang
# Usage:
#	docker build -f Dockerfile -t fedlang/vm .
#	docker run -it fedlang/vm bash
# 
# Inside the docker container, open 4 terminals using tmux:
#
# Terminal 1 - To run the director:
# 
# $ pipenv shell
# $ sh run_director.sh
# 
# Terminal 2 - To run a client 0:
# 
# $ pipenv shell
# $ sh run_clients.sh 0 0
# 
# Terminal 3 - To run a client 1:
# 
# $ pipenv shell
# $ sh run_clients.sh 1 1
# 
# Terminal 4 - To run a federation:
# 
# $ erlc stats_node.erl 
# $ erl -name stats_node_test@127.0.0.1 -setcookie "cookie_123456789" -kernel prevent_overlapping_partitions false
# 
# Inside erlang interpreter run:
# 
# stats_node:run_and_monitor("mboxDirector", "director@127.0.0.1", {"fcmeans", python, 1, 1, 2, "max_number_rounds", none, 10, "{  \"algorithm\": \"fcmeans\",  \"clientSelectionStrategy\": 1,  \"minNumberClients\": 2,  \"stopCondition\": null,  \"stopConditionThreshold\": null,  \"maxNumberOfRounds\": 10,  \"parameters\": {     \"numFeatures\": 16,     \"numClusters\": 10,     \"targetFeature\": 16,     \"lambdaFactor\": 2,     \"seed\": 10  }}"}).
