#! /bin/sh
docker.exe run -it --rm -v .:/app/fedlang fedlang/vm bash -c "
	tmux new -d 'pipenv run sh run_director.sh'; sleep 1; 
	tmux split-window 'pipenv run sh run_clients.sh 0 0'; 
	tmux split-window 'pipenv run sh run_clients.sh 1 1'; sleep 1; 
	tmux split-window 'erlc stats_node.erl && erl -name stats_node_test@127.0.0.1 -setcookie 'cookie_123456789' -kernel prevent_overlapping_partitions false -noshell -s stats_node test -s init stop'; 
	tmux attach
"
