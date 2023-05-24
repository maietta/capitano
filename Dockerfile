FROM node:18-alpine

ENV ORIGIN=${ORIGIN}
ENV NODE_ENV production

# Capitano is installed in /usr/bin/capitano

WORKDIR /app
COPY capitano /app/capitano
RUN chmod +x /app/capitano

WORKDIR /app

EXPOSE 80

ENTRYPOINT ["/app/capitano"]
