import os
import traceback
import time
import pickle
import random
import numpy as np
from functools import wraps
from typing import Final, Callable
from python_common.logging_utils import CustomLogger


log_file_name = f"client_{os.environ['FL_CLIENT_ID']}.log"
logger = CustomLogger(log_file_name, os.environ['FL_CLIENT_LOG_FOLDER'])
logger_info: Callable = logger.info
logger_error: Callable = logger.error
logger_info(f'Logging started in client.')


UTF_8: Final[str] = "utf-8"


def getCurrentMemoryUsage():
    """ Memory usage in kB """

    with open('/proc/self/status') as f:
        memusage = f.read().split('VmRSS:')[1].split('\n')[0][:-3]

    return int(memusage.strip()) / 1024


def timeit(method):
    @wraps(method)
    def timed(*args, **kw):
        ts = time.time()
        result = method(*args, **kw)
        te = time.time()
        if 'log_time' in kw:
            name = kw.get('log_name', method.__name__.upper())
            kw['log_time'][name] = int((te - ts) * 1000)
        else:
            logger.info('Performance metrics method = %r, execution time: %2.2f ms' % \
                  (method.__name__, (te - ts) * 1000))
        return result
    return timed


class FLExperiment:
    def __init__(self, max_number_rounds, client_ids, validate=False):
        self._global_model_parameters = None
        self._client_configuration = None
        self._max_number_rounds = max_number_rounds
        self._client_ids = client_ids
        # type of termination-related state variables
        self._type_of_termination = None
        self._termination_threshold = None
        self._monitored_metric = None
        # client selection-related state variables
        self._type_of_client_selection = None
        self._client_ratio = None
        self._client_values = None
        self._step_wise_client_selection = False
        self._client_selection_threshold = None
        self._round_selected_ids = None
        self._latency = False
        # communication paradigm-related state variables
        self._validate = validate
        self._calls_list = []

        self.experiment_history = {}

    def set_global_model_parameters(self, global_model_parameters):
        self._global_model_parameters = global_model_parameters

    def set_client_configuration(self, client_configuration):
        self._client_configuration = client_configuration

    def set_type_of_termination(self, type_of_termination, termination_threshold=None, monitored_metric=None):
        """
        type_of_termination should be a string equal custom, max_number_rounds, metric_under_threshold or metric_over_threshold
        When type_of_termination is metric_under_threshold or metric_over_threshold the threshold is passed
        as the termination threshold parameter, the monitored metric should be a key of the experiment_history dict, the
        relative entry should contain the values of the metric
        """
        self._type_of_termination = type_of_termination
        if type_of_termination == "metric_under_threshold" or type_of_termination == "metric_over_threshold":
            self._termination_threshold = float(termination_threshold)
            self._monitored_metric = monitored_metric

    def set_custom_communication_paradigm(self, round_structure):
        self._calls_list = [(bytes(side, 'utf-8'), bytes(function_name, 'utf-8')) for (side, function_name) in round_structure]

    def set_type_of_client_selection(self, type_of_client_selection, client_selection_ratio, client_values=None,
                                     threshold=None, step_client_selection=False):
        """
        type_of_client_selection should be a string equal custom, probability, ranking or threshold
        client_selection_ratio defines how many clients have to be selected at each round in case of probability sampling
        and ranking sampling, it defines the minimum number of clients to select for threshold sampling
        client_values should be a key of the experiment_history dict, the relative entry should contain the mapping between
        each client and relative value
        threshold is expected only with threshold sampling
        """
        self._type_of_client_selection = type_of_client_selection
        self._client_ratio = client_selection_ratio
        self._client_values = client_values
        self._step_wise_client_selection = step_client_selection
        self._client_selection_threshold = threshold

    def latency_required(self, latency_required=False):
        self._latency = latency_required

    def _get_current_value_monitored_metric(self):
        """
        The method returns the current value of the monitored metric stored in the dictionary experiment_history.
        The monitored metric entry can be stored as a single value or as a list.
        When it is a list, it is expected that the current (most recent) value is the last of the list
        """
        key = self._monitored_metric
        if key in self.experiment_history:
            if type(self.experiment_history[key]) == list:
                return float(self.experiment_history[key][-1])
            else:
                return float(self.experiment_history[key])

    def _get_current_clients_selection_ratio(self):
        if type(self._client_ratio) == str:
            current_client_selection_ratio = self.experiment_history[self._client_ratio]
        else:
            current_client_selection_ratio = self._client_ratio
        return current_client_selection_ratio

    def _get_current_cs_threshold(self):
        if type(self._client_selection_threshold) == str:
            current_cs_threshold = self.experiment_history[self._client_selection_threshold]
        else:
            current_cs_threshold = self._client_selection_threshold
        return current_cs_threshold

    def _perform_client_selection(self):
        """
            Performs client selection and returns IDs of the selected clients
        """
        if self._type_of_client_selection is None:
            # no selection, default value
            selected_ids = self._client_ids
            return selected_ids

        elif self._type_of_client_selection == "probability":
            """
                probability sampling
            """
            clients_to_select = round(len(self._client_ids) * self._get_current_clients_selection_ratio())
            if clients_to_select < 1:
                clients_to_select = 1

            if self._client_values is None:
                selected_ids = random.sample(self._client_ids, clients_to_select)
            else:
                key = self._client_values
                values_dict = self.experiment_history[key]
                ids = list(values_dict.keys())
                values = list(values_dict.values())
                if not ids:
                    selected_ids = random.sample(self._client_ids, clients_to_select)
                else:
                    # check ids with client_ids?
                    # weighted sampling
                    selected_ids = list(np.random.choice(ids, clients_to_select, p=values, replace=False))
            return selected_ids
        elif self._type_of_client_selection == "ranking":
            """
                ranking sampling
            """
            clients_to_select = round(len(self._client_ids) * self._get_current_clients_selection_ratio())

            key = self._client_values
            values_dict = self.experiment_history[key]
            tuple_list = [(client_id, value) for client_id, value in values_dict.items()]
            # sort clients by value in descending order
            tuple_list.sort(key=lambda tup: tup[1], reverse=True)
            # select top "clients_to_select" clients on the ranking
            selected_ids = [client_id for client_id, value in tuple_list[:clients_to_select]]

            return selected_ids
        elif self._type_of_client_selection == "threshold":
            """
                threshold sampling
            """

            min_number_of_clients = round(len(self._client_ids) * self._get_current_clients_selection_ratio())
            threshold = self._get_current_cs_threshold()

            key = self._client_values
            values_dict = self.experiment_history[key]

            tuple_list = [(client_id, value) for client_id, value in values_dict.items()]
            # sort clients by value in descending order
            tuple_list.sort(key=lambda tup: tup[1], reverse=True)

            # select top "min_number_of_clients" clients on the ranking
            selected_ids = [client_id for client_id, value in tuple_list[:min_number_of_clients]]

            # select the other clients that overcome the threshold
            for client_id, value in tuple_list[min_number_of_clients:len(tuple_list)]:
                if value > threshold:
                    selected_ids.append(client_id)
                else:
                    # tuple_list is sorted, when a client is not over the threshold all the subsequent won't be as well
                    return selected_ids

            return selected_ids

        elif self._type_of_client_selection == "custom":
            """
                custom sampling
            """

            key = self._client_values
            values_dict = self.experiment_history[key]

            # selected clients have value True, non-selected clients have value False
            selected_ids = [client_id for client_id, value in values_dict.items() if value is True]

            return selected_ids

    def get_initialization(self):
        """
            Erlang middleware knows the order of the parameters, and will use the first to pass to the clients,
            the second and the third (and fourth if any) to implement middleware support for termination condition
        """
        # conversion from Python str to bytes
        type_of_termination = bytes(self._type_of_termination, 'utf-8')

        if not self._calls_list:
            if self._validate:
                self._calls_list = [(bytes('standard', 'utf-8'), bytes('two_step', 'utf-8'))]
            else:
                self._calls_list = [(bytes('standard', 'utf-8'), bytes('one_step', 'utf-8'))]
        # else: self._calls_list contains the custom list of calls to be called at each round

        return_value = (self._client_configuration, self._max_number_rounds,
                        type_of_termination, self._termination_threshold,
                        self._latency, self._calls_list)

        return return_value

    def get_round_parameters(self):
        """
            The function is called at the beginning of the round, selects the clients of the round and
            the current global parameters
        """
        self._round_selected_ids = self._perform_client_selection()
        return self._global_model_parameters, self._round_selected_ids

    def get_step_data(self, parameters=None):
        """
        Utility function for communication paradigms with multiple steps
        """
        if self._step_wise_client_selection is False:
            step_selected_ids = self._round_selected_ids
        else:
            step_selected_ids = self._perform_client_selection()
        if parameters is None:
            return self._global_model_parameters, step_selected_ids
        else:
            return parameters, step_selected_ids

    def get_round_results(self):
        """
        Depending on the type of termination condition, the format of the results is different
        """
        if self._type_of_termination == "metric_under_threshold" or self._type_of_termination == "metric_over_threshold":
            current_value_monitored_metric = self._get_current_value_monitored_metric()
            return current_value_monitored_metric
