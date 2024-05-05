#!/usr/bin/env python3
# -*- coding: utf-8 -*-
# Copyright (C) 2024 AI&RD Research Group, Department of Information Engineering, University of Pisa
# SPDX-License-Identifier: Apache-2.0

import pickle
from pickle import HIGHEST_PROTOCOL
from timeit import default_timer as timer
from memory_profiler import profile
import gc
import os
import traceback
import sys
import json
import _pickle as cPickle
import numpy as np
from term import Atom
from functools import reduce
import tensorflow.keras as ke
from tensorflow.keras import Sequential
from tensorflow.keras.layers import Conv2D
from tensorflow.keras.layers import Dense
from tensorflow.keras.layers import Flatten
import python_common.common_utils as fl_utils
from python_common.common_utils import logger_info, logger_error
from python_common.fedlang_process import start_process, FedLangProcess, get_object_size
from python_common.common_utils import getCurrentMemoryUsage

runtime_file: str = os.environ.get("RUNTIME_FILE")
metric_file = open(str(os.environ.get("METRIC_FILE")), 'w')
start_fl_time = None


def init_server(client_ids):
    max_number_rounds = 10

    experiment = fl_utils.FLExperiment(max_number_rounds, client_ids, validate=True)
    experiment.set_type_of_termination('max_number_rounds', max_number_rounds)
    experiment.set_type_of_client_selection('probability', 1.0)
    experiment.set_client_configuration({
        "batch_size": 32,
        "local_epochs": 1,
        "val_steps": 1,
        "total_number_of_clients": 40
    })
    experiment.latency_required(True)
    experiment.experiment_history['accuracy'] = []
    experiment.experiment_history['loss'] = []
    experiment.set_global_model_parameters(init_model())
    try:
        pass
    except:
        logger_error(traceback.format_exc())
    return experiment


@profile(stream=metric_file)
def process_server(experiment, client_responses):
    start_time = timer()
    n_total_examples = sum([data[0] for _, data in client_responses])

    # compute weighted weights from all the clients
    weighted_weights = [[layer_weights * data[0] for layer_weights in data[1]] for _, data in client_responses]

    global_weights: np.ndarray = [
        reduce(np.add, layer_updates) / n_total_examples
        for layer_updates in zip(*weighted_weights)
    ]

    del weighted_weights

    try:
        experiment.set_global_model_parameters(global_weights)
    except:
        logger_error(traceback.format_exc())
    finally:
        duration = timer() - start_time
        with open(runtime_file, 'a') as file:
            file.write(f'{start_time - start_fl_time}, {duration}\n')
    return experiment.get_step_data(global_weights)


def validate_server(experiment, client_responses):

    n_total_examples = sum([data[0] for _, data in client_responses])

    weighted_accuracies = [data[0]*data[1] for _, data in client_responses]
    weighted_losses = [data[0]*data[2] for _, data in client_responses]

    new_accuracy = sum(weighted_accuracies)/n_total_examples
    new_loss = sum(weighted_losses)/n_total_examples

    experiment.experiment_history['accuracy'].append(new_accuracy)
    experiment.experiment_history['loss'].append(new_loss)
    logger_info(f'new_accuracy: {new_accuracy:.3f}, new_loss: {new_loss:.3f}')
    return experiment.get_round_results()


class MNISTServer(FedLangProcess):

    def __init__(self, **kwargs):
        super().__init__(**kwargs)

    def init_server(self, experiment, config_file, bb):
        logger_info(f'inside init_server async execution')

        bb = bytes(bb, encoding='utf8')
        client_ids = list(bb)
        logger_info(
            f"experiment = {experiment}, config_file = {config_file}, client_ids = {client_ids}")
        logger_info('before init_server')
        self.experiment = init_server(client_ids)
        logger_info('after init_server')
        logger_info('before get_initialization')
        client_config, max_num_rounds, termination_type, threshold_value, latency, calls_list = self.experiment.get_initialization()
        logger_info('after get_initialization')
        fl_utils.logger.info(f"before sending  ({erl_worker_mailbox}, {erl_client_name} )! (fl_server_ready)")
        client_configuration_str = json.dumps(client_config)
        logger_info(f'client_config = {client_config}')
        self.get_node().send_nowait(sender=self.pid_,
                            receiver=(Atom(erl_client_name), Atom(erl_worker_mailbox)),
                            message=(Atom('fl_server_ready'), self.pid_, client_configuration_str, calls_list))
        logger_info(f"after sending  ({erl_worker_mailbox}, {erl_client_name} )! (fl_server_ready)")

    def start_round(self, round_mail_box, experiment, round):
        global start_fl_time
        if start_fl_time is None:
            start_fl_time = timer()
        result, client_ids = self.experiment.get_round_parameters()
        logger_info(f"before sending  ({erl_worker_mailbox}, {erl_client_name} )! (start_round_ok)")

        tt = cPickle.dumps(result, protocol=HIGHEST_PROTOCOL)

        self.get_node().send_nowait(sender=self.pid_,
                            receiver=(Atom(erl_client_name), Atom(round_mail_box)),
                            message=(Atom('start_round_ok'), tt, client_ids))

        logger_info(f"after sending  ({erl_worker_mailbox}, {erl_client_name} )! (start_round_ok)")

    def process_server(self, round_mail_box, experiment, round_it, client_responses):
        try:

            logger_info(f"start process_server, experiment = {experiment}, round_mail_box = {round_mail_box}")
            data = [(client_response[0], cPickle.loads(bytes(client_response[1]))) for client_response in
                    client_responses]
            logger_info(f'=== sys.getsizeof data = {get_object_size(data)}\n')
            data, client_ids = process_server(self.experiment, data)

            send_task = self.get_node().send(sender=self.pid_,
                         receiver=(Atom(erl_client_name), Atom(round_mail_box)),
                         message=(Atom('process_server_ok'), pickle.dumps(data), client_ids))

            def when_finished(_fut):
                logger_info(f"MEMORY IN AGGREGATOR {getCurrentMemoryUsage()}")

            send_task = self.get_node().get_loop().create_task(send_task)
            send_task.add_done_callback(when_finished)

            logger_info(f"end process_server, experiment = {experiment}, round_it = {round_it}, sys.getsizeof = {sys.getsizeof(self.experiment)}")
        except:
            logger_error(traceback.format_exc())

    def validate_server(self, round_mail_box, experiment, round_number, client_responses):
        try:
            logger_info(
                f"start validate_server, experiment = {experiment}, round_mail_box = {round_mail_box}, round_number = {round_number}")
            data = [(client_response[0], cPickle.loads(bytes(client_response[1]))) for client_response in
                    client_responses]
            to_return = validate_server(self.experiment, data)
            self.get_node().send_nowait(sender=self.pid_,
                                    receiver=(Atom(erl_client_name), Atom(round_mail_box)),
                                    message=(Atom('validate_server_ok'), to_return, "{}"))
            logger_info(f"end validate_server, experiment = {experiment}, round_number = {round_number}")
        except:
            logger_error(traceback.format_exc())

    def finish(self):
        metric_file.close()
        FedLangProcess.exit(self, None)
        logger_info('DESTROYYY')
        sys.exit(0)


def init_model():
    input_shape = (28, 28, 1)
    num_classes = 10
    conv_kernel_size = (4, 4)
    conv_strides = (2, 2)
    conv1_channels_out = 16
    conv2_channels_out = 32
    final_dense_inputsize = 100

    model = Sequential()

    model.add(Conv2D(conv1_channels_out,
                     kernel_size=conv_kernel_size,
                     strides=conv_strides,
                     activation='relu',
                     input_shape=input_shape))

    model.add(Conv2D(conv2_channels_out,
                     kernel_size=conv_kernel_size,
                     strides=conv_strides,
                     activation='relu'))

    model.add(Flatten())

    model.add(Dense(final_dense_inputsize, activation='relu'))

    model.add(Dense(num_classes, activation='softmax'))

    model.compile(loss=ke.losses.categorical_crossentropy,
                  optimizer=ke.optimizers.legacy.Adam(),
                  metrics=['accuracy'])

    return model.get_weights()


if __name__ == "__main__":
    pyrlang_node_id = sys.argv[1]
    erl_client_name = sys.argv[2]
    erl_worker_mailbox = sys.argv[3]
    erl_cookie = sys.argv[4]
    experiment_id = sys.argv[5]
    logger_info(f'pyrlang_node_id = {pyrlang_node_id}, '
                f'erl_client_name = {erl_client_name}, '
                f'erl_worker_mailbox = {erl_worker_mailbox}, '
                f'erl_cookie = {erl_cookie}')

    gc.collect()
    start_process(MNISTServer, pyrlang_node_id, erl_cookie, erl_client_name, erl_worker_mailbox)
