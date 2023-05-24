FROM node:18-alpine

ENV ORIGIN=${ORIGIN}
ENV NODE_ENV production

ADD https://github.com/maietta/capitano/releases/download/latest/capitano_cgo_linux_amd64 /usr/bin/capitano
RUN chmod +x /app/capitano

WORKDIR /app

EXPOSE 80

ENTRYPOINT ["/app/capitano"]
