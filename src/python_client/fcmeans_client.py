#!/usr/bin/env python3
# -*- coding: utf-8 -*-
# Copyright (C) 2024 AI&RD Research Group, Department of Information Engineering, University of Pisa
# SPDX-License-Identifier: Apache-2.0

import math
import os
import sys
import json
import time
import psutil
import traceback
import pandas as pd
import numpy as np
import _pickle as cPickle
from term import Atom
import python_common.common_utils as fl_utils
from numba import jit
from sklearn.metrics import adjusted_rand_score as ARI
from python_common.fedlang_process import FedLangProcess, start_process
from python_common.common_utils import logger_info, logger_error

FROBENIUS_NORM = 'fro'

start_time_fl = None
process = psutil.Process()

@jit(nopython=True)
def distance_fn(u: np.ndarray, v: np.ndarray):
    return np.linalg.norm(u - v)


def load_experiment_info(num_clients, target_feature, dataset_name = None):
    base_path = os.path.join(os.environ.get("PROJECT_PATH"), 'datasets')
    if dataset_name is None:
        dataset_name = 'pendigits.csv'
    dataset_path = os.path.join(base_path, dataset_name)
    df = pd.read_csv(dataset_path)
    size = int(math.ceil(df.shape[0] / num_clients))
    logger_info(f'dataset chunk size = {size}')
    y_true = df[str(target_feature)]
    df_X = df.drop(str(target_feature), axis=1)
    random_samples = np.random.randint(df.shape[0], size=size)
    values = df_X.values[random_samples, :]
    y_true = y_true.values[random_samples]
    logger_info(f'dataset chunk size = {size}, X.shape = {df_X.shape}, y.shape = {y_true.shape}')
    return values, y_true, target_feature


class FCMeansClientProcess(FedLangProcess):

    def __init__(self, **kwargs):
        super().__init__(**kwargs)

    def init_client(self, experiment, json_str_config: str):
        try:
            experiment_config = json.loads(json_str_config)
            fl_utils.logger.info(f"experiment = {experiment}, experiment_config = {experiment_config}")
            self.factor_lambda = float(experiment_config.get("lambdaFactor"))
            self.num_clients = experiment_config.get("numClients")
            self.target_feature = experiment_config.get("targetFeature")
            dataset_name = experiment_config.get("datasetName")
            self.X, self.y_true, self.target_feature = load_experiment_info(self.num_clients, self.target_feature, dataset_name)
            self.rows, self.num_features = self.X.shape
            self.get_node().send_nowait(sender=self.pid_,
                                        receiver=(Atom(_erl_client_name), Atom(_erl_worker_mailbox)),
                                        message=(Atom('fl_client_ready'), self.pid_))
            logger_info(f"after sending  ({_erl_worker_mailbox}, {_erl_client_name} )! (fl_client_ready)")
        except:
            logger_error(traceback.format_exc())

    def process_client(self, experiment, round_number, centers_param):
        global start_time_fl
        start_time = time.time()

        data = None

        try:
            logger_info(f"start process_client, experiment = {experiment}, round = {round_number}")
            centers_list = cPickle.loads(bytes(centers_param))
            #fl_utils.logger.info(f'{type(centers_list)} centers_list = {centers_list}')
            centers = np.array(centers_list)
            num_clusters = len(centers)
            num_objects = len(self.X)
            factor_lambda = self.factor_lambda

            ws = [[0] * self.num_features for i in range(num_clusters)]
            u = [0] * num_clusters

            for i in range(num_objects):
                denom = 0
                numer = [0] * num_clusters
                x = self.X[i]

                for c in range(num_clusters):
                    vc = centers[c]
                    numer[c] = (distance_fn(x, vc)) ** ((2) / (factor_lambda - 1))
                    if numer[c] == 0:
                        numer[c] = np.finfo(np.float).eps
                    denom = denom + (1 / numer[c])

                for c in range(num_clusters):
                    u_c_i = (numer[c] * denom) ** -1
                    ws[c] = ws[c] + (u_c_i ** factor_lambda) * x
                    u[c] = u[c] + (u_c_i ** factor_lambda)
            ws = [item.tolist() for item in ws]
            data = (u, ws)

            u = [[0] * num_objects for i in range(num_clusters)]

            for i in range(num_objects):

                denom = 0
                numer = [0] * num_clusters
                x = self.X[i]

                for c in range(num_clusters):
                    vc = centers[c]

                    numer[c] = (distance_fn(x, vc)) ** ((2) / (factor_lambda - 1))
                    if numer[c] == 0:
                        numer[c] = np.finfo(np.float64).eps
                    denom = denom + (1 / numer[c])

                for c in range(num_clusters):
                    u_c_i = (numer[c] * denom) ** -1
                    u[c][i] = u_c_i

            u = np.asarray(u).T
            y_pred = np.argmax(u, 1)

            ari_client = ARI(self.y_true, y_pred)

            total_memory, used_memory, free_memory = map(int, os.popen('free -t -m').readlines()[-1].split()[1:])
            memory_usage_percentage = round((used_memory/total_memory) * 100, 2)
            cpu_usage_percentage = psutil.cpu_percent()
            metrics_message = {
                "timestamp": int(time.time()), "type": "worker_metrics",
                "round": round_number,
                "client_id": os.environ.get("FL_CLIENT_ID"),
                "hostMetrics": {
                                "cpuUsagePercentage": cpu_usage_percentage,
                                "memoryUsagePercentage": memory_usage_percentage
                },
                "modelMetrics": {"ARI": ari_client}
            }

            self.get_node().send_nowait(sender=self.pid_,
                                        receiver=(Atom(_erl_client_name), Atom(_erl_worker_mailbox)),
                                        message=(Atom('fl_py_result'), cPickle.dumps(data), json.dumps(metrics_message)))
            logger_info(f"end process_client, experiment = {experiment}, round = {round_number}")
        except:
            logger_error(traceback.format_exc())

    def destroy(self):
        FedLangProcess.exit(self, None)
        logger_info('DESTROYYY')
        sys.exit(0)


if __name__ == "__main__":
    _pyrlang_node_id = sys.argv[1]
    _erl_client_name = sys.argv[2]
    _erl_worker_mailbox = sys.argv[3]
    _erl_cookie = sys.argv[4]
    experiment_id = sys.argv[5]
    client_id = os.environ.get("FL_CLIENT_ID")
    logger_info(f'pyrlang_node_id = {_pyrlang_node_id}, '
                         f'erl_client_name = {_erl_client_name}, '
                         f'erl_worker_mailbox = {_erl_worker_mailbox}, '
                         f'erl_cookie = {_erl_cookie}, '
                         f'experiment_id = {experiment_id}, '
                         f'client_id = {client_id}')
    start_process(FCMeansClientProcess, _pyrlang_node_id, _erl_cookie, _erl_client_name, _erl_worker_mailbox)
