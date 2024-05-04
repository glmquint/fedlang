#!/usr/bin/env python3
# -*- coding: utf-8 -*-
"""
Created on Apr. 27 9:27 a.m. 2024

@author: José Luis Corcuera Bárcena
"""
import gc
import sys
import json
import os
import _pickle as cPickle
from term import Atom
from pyrlang.node import Node
from pyrlang.process import Process
import tensorflow.keras as ke
from tensorflow.keras import Sequential
from tensorflow.keras.layers import Conv2D
from tensorflow.keras.layers import Dense
from tensorflow.keras.layers import Flatten
import python_common.common_utils as fl_utils
from python_common.fedlang_process import FedLangProcess, start_process
from python_common.mnist_utils import load_mnist_shard
from pickle import HIGHEST_PROTOCOL
from memory_profiler import profile
from python_common.common_utils import logger_info, logger_error

os.environ["TF_CPP_MIN_LOG_LEVEL"] = "3"


def init_client(params):
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

    client_number = int(os.getenv("FL_CLIENT_ID"))
    total_number_of_clients = params.get('total_number_of_clients')

    logger_info(f'client_number = {client_number}, total_number_of_clients = {total_number_of_clients}')

    _, num_classes, X_train, y_train, X_valid, y_valid = load_mnist_shard(
        shard_num=client_number, collaborator_count=total_number_of_clients
    )
    logger_info(f'X_train = {X_train.shape}, X_valid = {X_valid.shape}')
    logger_info(f'y_train = {y_train.shape}, y_valid = {y_valid.shape}')
    logger_info(f'y_valid = {y_valid}')
    return model, X_train, y_train, X_valid, y_valid, params


@profile(stream=open(os.environ['METRIC_FILE'], 'w'))
def process_client(model, X_train, y_train, params, global_weights):
    model.set_weights(global_weights)
    model.fit(
        X_train,
        y_train,
        params.get("batch_size"),
        params.get("local_epochs"),
        validation_split=0.1,
    )
    gc.collect()
    return len(X_train), model.get_weights()


def validate_client(model, x_test, y_test, config, global_weights):
    """Evaluate parameters on the locally held test set."""

    # Update local model with global parameters
    model.set_weights(global_weights)

    # Get config values
    steps: int = config["val_steps"]

    # Evaluate global model parameters on the local test data and return results
    loss, accuracy = model.evaluate(x_test, y_test, 32, steps=steps)
    gc.collect()
    num_examples_test = len(x_test)
    logger_info(f'num_examples_test = {num_examples_test}, accuracy = {accuracy}, loss = {loss}')
    return num_examples_test, accuracy, loss


class MNISTClient(FedLangProcess):
    def __init__(self, **kwargs):
        super().__init__(**kwargs)

    def init_client(self, experiment, json_str_config: str):
        experiment_config = json.loads(json_str_config)
        logger_info(f"experiment = {experiment}, experiment_config = {experiment_config}")

        model, X_train, y_train, X_valid, y_valid, params = init_client(experiment_config)
        self.model = model
        self.X_train = X_train
        self.y_train = y_train
        self.X_valid = X_valid
        self.y_valid = y_valid
        self.params = experiment_config
        logger_info(f"before sending  ({self.erl_worker_mailbox}, {self.erl_client_name} )! (fl_client_ready)")
        self.get_node().send_nowait(sender=self.pid_,
                                    receiver=(Atom(self.erl_client_name), Atom(self.erl_worker_mailbox)),
                                    message=(Atom('fl_client_ready'), self.pid_))
        logger_info(f"after sending  ({self.erl_worker_mailbox}, {self.erl_client_name} )! (fl_client_ready)")

    def process_client(self, experiment, current_round, global_weights_bytes):
        logger_info(f'experiment = {experiment}, current_round = {current_round}')
        global_weights = cPickle.loads(bytes(global_weights_bytes))
        logger_info(f'global_weights.size = {len(global_weights)}')
        nrows, new_weights = process_client(self.model, self.X_train, self.y_train, self.params, global_weights)
        logger_info('sending fl_py_result')
        self.get_node().send_nowait(sender=self.pid_,
                                    receiver=(Atom(self.erl_client_name), Atom(self.erl_worker_mailbox)),
                                    message=(
                                    Atom('fl_py_result'), cPickle.dumps((nrows, new_weights), HIGHEST_PROTOCOL), "{}"))
        logger_info('after sending fl_py_result')

    def validate_client(self, experiment, current_round, global_weights_bytes):
        global_weights = cPickle.loads(bytes(global_weights_bytes))
        num_examples_test, accuracy, loss = validate_client(self.model, self.X_valid, self.y_valid, self.params,
                                                            global_weights)
        logger_info('sending fl_py_result')
        self.get_node().send_nowait(sender=self.pid_,
                             receiver=(Atom(self.erl_client_name), Atom(self.erl_worker_mailbox)),
                             message=(Atom('fl_py_result'), cPickle.dumps((num_examples_test, accuracy, loss), HIGHEST_PROTOCOL), "{}"))
        logger_info('after fl_py_result')

    def destroy(self):
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
    start_process(MNISTClient, _pyrlang_node_id, _erl_cookie, _erl_client_name, _erl_worker_mailbox)
