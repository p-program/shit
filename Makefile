now    := $(shell date)
date  ?=  $(shell date "+%Y-%m-%d")


auto_commit:
	git add .
	# 需要注意的是，每行命令在一个单独的shell中执行。这些Shell之间没有继承关系。
	git commit -am "$(now)"
	git pull
	git push

run:
	docker compose up --build

build:
	docker compose build --no-cache

bbuild:
	go build