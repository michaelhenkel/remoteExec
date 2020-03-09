all: serverbinary clientbinary container
serverbinary:
	(cd server; GOOS=linux go build)
clientbinary:
	(cd client; go build)
container:
	docker build -t michaelhenkel/remotexec . && docker push michaelhenkel/remotexec
