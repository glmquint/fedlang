#!/usr/bin/env python3
# -*- coding: utf-8 -*-
# Copyright (C) 2024 AI&RD Research Group, Department of Information Engineering, University of Pisa
# SPDX-License-Identifier: Apache-2.0

import os
import sys
import traceback
import time
import numpy as np
import _pickle as cPickle
from term import Atom
from typing import Final
import python_common.common_utils as fl_utils
from python_common.common_utils import logger_info, logger_error
from timeit import default_timer as timer
import json
import psutil

from python_common.fedlang_process import FedLangProcess, start_process

LOG_FOLDER_PATH: Final[str] = os.environ.get("FL_CLIENT_LOG_FOLDER")
FROBENIUS_NORM: Final[str] = 'fro'

start_fl_time = None

process = psutil.Process()


def norm_fro(u: np.ndarray):
    return np.linalg.norm(u, ord=FROBENIUS_NORM)


class FCMeansServerProcess(FedLangProcess):

    def __init__(self, **kwargs):
        super().__init__(**kwargs)

    def init_server(self, experiment: str, json_str_config: str, bb):
        try:
            bb = bytes(bb, encoding='utf8')
            client_ids = list(bb)
            experiment_config = json.loads(json_str_config)
            logger_info(f"experiment = {experiment}, client_ids = {client_ids}, experiment_config = {experiment_config}")
            parameters = experiment_config.get("parameters")
            n_features = int(parameters.get("numFeatures"))
            seed = int(parameters.get("seed"))
            n_clusters = int(parameters.get("numFeatures"))
            self.target_feature = parameters.get("targetFeature")
            self.max_number_rounds = int(experiment_config.get("maxNumberOfRounds"))
            lambda_factor = int(parameters.get("lambdaFactor"))
            _stop_condition_threshold = experiment_config.get("stopConditionThreshold")
            if _stop_condition_threshold is not None:
                self.epsilon = int(_stop_condition_threshold)
            else:
                self.epsilon = None
            logger_info(f"n_clusters = {n_clusters}, epsilon = {self.epsilon}, max_number_rounds = {self.max_number_rounds}, n_features = {n_features}")
            min_num_clients = int(experiment_config.get("minNumberClients"))
            experiment = fl_utils.FLExperiment(self.max_number_rounds, client_ids)
            experiment.set_client_configuration({
                'lambdaFactor': lambda_factor,
                'numClients': min_num_clients,
                'targetFeature': self.target_feature,
                'do_sleep': True
            })
            experiment.latency_required(True)
            np.random.seed(seed)
            centers = np.random.rand(n_clusters, n_features)
            self.num_clusters = n_clusters
            self.num_features = n_features
            cluster_centers = list()
            cluster_centers.append(centers)
            self.cluster_centers = cluster_centers
            self.fnorms = list()
            experiment.set_global_model_parameters([center.tolist() for center in centers])
            logger_info(f'centers = {centers}')
            experiment.set_type_of_termination('custom')
            experiment.set_type_of_client_selection('probability', 1.0)
            self.experiment = experiment
            self.current_round = 0
            client_config, max_num_rounds, termination_type, threshold_value, latency, calls_list = self.experiment.get_initialization()
            client_configuration_str = json.dumps(client_config)
            logger_info(f"before sending  ({erl_worker_mailbox}, {erl_client_name} )! (fl_server_ready)")
            self.get_node().send_nowait(sender=self.pid_,
                                        receiver=(Atom(erl_client_name), Atom(erl_worker_mailbox)),
                                        message=(Atom('fl_server_ready'), self.pid_, client_configuration_str, calls_list))
            logger_info(f"after sending  ({erl_worker_mailbox}, {erl_client_name} )! (fl_server_ready)")
        except:
            logger_error(traceback.format_exc())

    def start_round(self, round_mail_box, experiment, round_number):
        global start_fl_time
        if start_fl_time is None:
            start_fl_time = timer()
        result, client_ids = self.experiment.get_round_parameters()
        logger_info(f"start_round result = {result}")
        logger_info(f"before sending  ({erl_worker_mailbox}, {erl_client_name} )! (start_round_ok)")
        tt = cPickle.dumps(result)
        self.round_number = round_number
        self.get_node().send_nowait(sender=self.pid_,
                                    receiver=(Atom(erl_client_name), Atom(round_mail_box)),
                                    message=(Atom('start_round_ok'), tt, client_ids))
        logger_info(f"after sending  ({erl_worker_mailbox}, {erl_client_name} )! (start_round_ok)")

    #@profile(stream=open(str(os.environ.get("METRIC_FILE")), 'w'))
    def process_server(self, round_mail_box, experiment, config_file, client_responses):
        start_time = timer()
        try:
            time_to_sleep = np.random.uniform(2, 4)
            #time.sleep(time_to_sleep)
            logger_info(f"start process_server, experiment = {experiment}, round_mail_box = {round_mail_box}, len(client_responses) = {len(client_responses)}")
            data = [(client_response[0], cPickle.loads(bytes(client_response[1]))) for client_response in
                    client_responses]
            for i in range(len(data)):
                fl_utils.logger.info(f'i = {i}, data[0] = {data[i][0]}')

            client_responses = [cr[1] for cr in data]

            self.current_round = self.current_round + 1
            num_clients = len(client_responses)

            u_list = [0] * self.num_clusters
            ws_list = [[0] * self.num_features for i in range(self.num_clusters)]

            for client_idx in range(num_clients):
                # remember the response is a list of tuples where each tuple represents the (u, ws) for each cluster
                response = client_responses[client_idx]
                for i in range(self.num_clusters):
                    client_u = response[0][i]
                    client_ws = response[1][i] if response[1][i] is np.array else np.array(response[1][i])
                    u_list[i] = u_list[i] + client_u
                    ws_list[i] = ws_list[i] + client_ws

            new_cluster_centers = []
            prev_cluster_centers = self.cluster_centers[-1]

            for i in range(self.num_clusters):
                u = u_list[i]
                ws = ws_list[i]
                if (u == 0):
                    center = prev_cluster_centers[i]
                else:
                    center = ws / (u * 1.0)
                new_cluster_centers.append(np.array(center))

            self.experiment.set_global_model_parameters([centers.tolist() for centers in new_cluster_centers])
            self.cluster_centers.append(new_cluster_centers)

            data, client_ids = self.experiment.get_step_data([centers.tolist() for centers in new_cluster_centers])
            #logger_info(f"process_server, data = {data}, client_ids = {client_ids}")

            centers_r = np.array(self.cluster_centers[-1])
            centers_r_1 = np.array(self.cluster_centers[-2])
            tot_diff_sum = norm_fro(centers_r - centers_r_1)

            total_memory, used_memory, free_memory = map(int, os.popen('free -t -m').readlines()[-1].split()[1:])
            memory_usage_percentage = round((used_memory/total_memory) * 100, 2)
            cpu_usage_percentage = psutil.cpu_percent()
            metrics_message = {
                "timestamp": int(time.time()), "type": "strategy_server_metrics",
                "round": self.current_round,
                "hostMetrics": {
                                "cpuUsagePercentage": cpu_usage_percentage,
                                "memoryUsagePercentage": memory_usage_percentage
                },
                "modelMetrics": {"FRO": tot_diff_sum}
            }

            self.get_node().send_nowait(sender=self.pid_,
                                        receiver=(Atom(erl_client_name), Atom(round_mail_box)),
                                        message=(Atom('process_server_ok'), cPickle.dumps(data), json.dumps(metrics_message)))
            logger_info(f"end process_server, experiment = {experiment}, round = {config_file}")

        except:
            logger_error(traceback.format_exc())

    def next_round(self):
        try:
            num_centers = len(self.cluster_centers)
            if num_centers > 1:
                centers_r = np.array(self.cluster_centers[-1])
                centers_r_1 = np.array(self.cluster_centers[-2])
                tot_diff_sum = norm_fro(centers_r - centers_r_1)
                self.fnorms.append(tot_diff_sum)

            to_return = self.round_number < self.max_number_rounds
            self.get_node().send_nowait(sender=self.pid_,
                                        receiver=(Atom(erl_client_name), Atom(erl_worker_mailbox)),
                                        message=(Atom('next_round_ok'), to_return))
            logger_info(f"end next_round, round_number = {self.round_number}, to_return = {to_return}")
        except:
            logger_error(traceback.format_exc())

    def get_current_metric_value(self):
        try:
            num_centers = len(self.cluster_centers)
            tot_diff_sum = self.epsilon
            if num_centers > 1:
                centers_r = np.array(self.cluster_centers[-1])
                centers_r_1 = np.array(self.cluster_centers[-2])
                tot_diff_sum = norm_fro(centers_r - centers_r_1)

            self.get_node().send_nowait(sender=self.pid_,
                                        receiver=(Atom(erl_client_name), Atom(erl_worker_mailbox)),
                                        message=(Atom('get_current_metric_value_ok'), float(tot_diff_sum)))
            logger_info(f"end get_current_metric_value, round_number = {self.round_number}, "
                        f"tot_diff_sum = {tot_diff_sum}, type(tot_diff_sum) = {type(tot_diff_sum)}")
        except:
            logger_error(traceback.format_exc())

    def finish(self):
        logger_info('DESTROY')
        self.exit()
        sys.exit(0)


if __name__ == "__main__":
    pyrlang_node_id = sys.argv[1]
    erl_client_name = sys.argv[2]
    erl_worker_mailbox = sys.argv[3]
    erl_cookie = sys.argv[4]
    experiment_id = sys.argv[5]
    runtime_file: str = os.path.join(LOG_FOLDER_PATH, f"time_by_method_aggregator_{experiment_id}.log")
    logger_info(f'pyrlang_node_id = {pyrlang_node_id}, '
                f'erl_client_name = {erl_client_name}, '
                f'erl_worker_mailbox = {erl_worker_mailbox}, '
                f'erl_cookie = {erl_cookie}')
    start_process(FCMeansServerProcess, pyrlang_node_id, erl_cookie, erl_client_name, erl_worker_mailbox)
