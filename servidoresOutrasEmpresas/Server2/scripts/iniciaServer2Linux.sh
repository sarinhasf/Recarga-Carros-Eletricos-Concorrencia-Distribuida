#!/bin/bash

docker-compose build
docker-compose create
docker ps -a
docker-compose start server2
docker-compose logs -f 





