#!/bin/bash

apt update -y
apt install -y docker.io docker-compose git

systemctl enable docker
systemctl start docker

git clone https://github.com/USERNAME/traffic-filter.git

cd traffic-filter

cp .env.example .env

docker compose up -d