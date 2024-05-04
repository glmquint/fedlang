#!/usr/bin/env python3
# -*- coding: utf-8 -*-
"""
Created on Sep 03 4:33 a.m. 2023

@author: jose 
"""
import os
import sys
import logging
import datetime


class CustomLogger(logging.getLoggerClass()):
    def __init__(self, name, log_dir=None):
        # Create custom logger logging all five levels
        super().__init__(name)
        self.setLevel(logging.DEBUG)

        # Create stream handler for logging to stdout (log all five levels)
        self.stdout_handler = logging.StreamHandler(sys.stdout)
        self.stdout_handler.setLevel(logging.DEBUG)
        self.stdout_handler.setFormatter(logging.Formatter('%(message)s'))

        # Create stream handler for logging to stdout (log all five levels)
        self.stderr_handler = logging.StreamHandler(sys.stderr)
        self.stderr_handler.setLevel(logging.ERROR)
        self.stderr_handler.setFormatter(logging.Formatter('%(message)s'))

        # Add file handler only if the log directory was specified
        self.file_handler = None
        if log_dir:
            self.add_file_handler(name, log_dir)

    def add_file_handler(self, name, log_dir):
        """Add a file handler for this logger with the specified `name` (and
        store the log file under `log_dir`)."""
        # Format for file log
        fmt = '%(asctime)s | %(levelname)8s | %(filename)s:%(lineno)d | %(message)s'
        formatter = logging.Formatter(fmt)

        log_name = name
        if not os.path.exists(log_dir):
            try:
                os.makedirs(log_dir)
            except:
                print('{}: Cannot create directory {}. '.format(
                    self.__class__.__name__, log_dir),
                    end='', file=sys.stderr)
                log_dir = '/tmp' if sys.platform.startswith('linux') else '.'
                print(f'Defaulting to {log_dir}.', file=sys.stderr)

        log_file = os.path.join(log_dir, log_name)

        # Create file handler for logging to a file (log all five levels)
        self.file_handler = logging.FileHandler(log_file)
        self.file_handler.setLevel(logging.DEBUG)
        self.file_handler.setFormatter(formatter)
        self.addHandler(self.file_handler)
