#!/usr/bin/env bash

cd /srv/app

gunicorn cla.routes:__hug_wsgi__ -b 0.0.0.0:5000 --log-level debug
