#!/usr/bin/env python3
# -*- coding: utf-8 -*-
"""
Created on Sep 19 4:01 a.m. 2023

@author: José Luis Corcuera Bárcena
"""
import gc
import os
import sys
import traceback
from term import Atom
from typing import Type, TypeVar
from pyrlang.node import Node
from pyrlang.process import Process as PyrlangProcess
from python_common.common_utils import logger_info, logger_error
from multiprocessing import Process

sys.stdout.flush()


def get_object_size(my_object):
    memory_size = 0
    ids = set()
    objects = [my_object]
    while objects:
        new = []
        for obj in objects:
            if id(obj) not in ids:
                ids.add(id(obj))
                memory_size += sys.getsizeof(obj)
                new.append(obj)
        objects = gc.get_referents(*new)
    return memory_size


class FedLangProcess(PyrlangProcess):

    def __init__(self, **kwargs) -> None:
        PyrlangProcess.__init__(self)
        logger_info("Registering process - 'my_process'")
        self.erl_client_name = kwargs.get("erl_client_name")
        self.erl_worker_mailbox = kwargs.get("erl_worker_mailbox")

    def handle_one_inbox_message(self, msg):
        try:
            pid = msg[0]
            fun_name = msg[1]
            self.sender = pid
            logger_info(f"sender = {pid}, fun_name = {fun_name}")
            func = getattr(self, fun_name)
            func(*msg[2:])
            # p = Process(target=func, args=tuple(msg[2:]))
            # p.start()
            logger_info(f'async call fun_name = {fun_name}')
        except Exception:
            logger_error(traceback.format_exc())


TFedLangProcess = TypeVar("TFedLangProcess", bound=FedLangProcess)


def start_process(class_to_instantiate: Type[TFedLangProcess],
                  node_name: str,
                  cookie: str,
                  client_name: str,
                  worker_mailbox: str):
    logger_info(f"node_name = {node_name}, erl_cookie = {cookie}")
    node = Node(node_name=node_name, cookie=cookie)
    event_loop = node.get_loop()
    instance = class_to_instantiate(erl_client_name=client_name, erl_worker_mailbox=worker_mailbox)
    logger_info(f'type(instance) = {type(instance)}')
    logger_info(f'dir(instance) = {dir(instance)}')
    def task():
        node.send_nowait(sender=instance.pid_,
                         receiver=(Atom(client_name), Atom(worker_mailbox)),
                         message=(Atom('pyrlang_node_ready'), instance.pid_, os.getpid()))
    event_loop.call_soon(task)
    node.run()
