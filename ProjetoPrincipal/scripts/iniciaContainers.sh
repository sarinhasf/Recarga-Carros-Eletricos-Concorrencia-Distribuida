#!/bin/bash

# Buildar as imagens
docker-compose build

# Criar os containers mas não iniciar
docker-compose create

# Caso queira já iniciar: descomente a linha abaixo
# docker-compose up -d

# Mostrar todos os containers
docker ps -a

# Inicia todos Services das empresas
docker-compose start server1 
docker-compose logs -f # Mostrar os logs em tempo real de todos os containers

# Inicia veiculo 1
#docker-compose start do cliente (veiculo)
#docker run -it inicio-de-um-sonho-client
#docker-compose start client
#docker exec -it client sh
#para rodar o codigo dentro do terminal do cliente, rode ./client

