[![build status](https://github.com/maietta/capitano/actions/workflows/release.yml/badge.svg)](https://github.com/maietta/capitano/actions/workflows/release.yml)

# Capitano: Deploying SvelteKit Websites with Pocketbase in a Docker Container

Capitano is a proof of concept created to address the need for deploying websites built with SvelteKit in SSR (Server-Side Rendering) mode while bundling a Pocketbase database and managing deployments within a single Docker container.

**Note: This project is currently in its early stages and may not be suitable for production use. Use it at your own discretion.**

## Motivation

The idea behind Capitano stemmed from a desire to simplify the deployment process for SvelteKit websites by incorporating a self-contained Docker container that includes both the website files and a Pocketbase database. This approach aims to streamline the deployment workflow and enhance portability.

## Disclaimer

While Capitano serves its intended purpose, please be aware that it is still a work in progress. It may undergo further refinement and improvements in the future. Consider this project as an experimental solution that was initially developed to meet specific project requirements.

## Contributing

If you find Capitano interesting and would like to contribute or provide feedback, please feel free to do so. Your input can help shape the future development of this project.

**Important: Use Capitano at your own risk, and always exercise caution when deploying applications in a production environment.**

## How to use in Docker?

```Dockerfile
FROM node:18-alpine

ENV ORIGIN=${ORIGIN}
ENV NODE_ENV production

ADD https://github.com/maietta/capitano/releases/download/untagged-f87df824ced0b3abdd36/capitano_1.0.1_linux_amd64 /usr/bin/capitano
RUN chmod +x /app/capitano

WORKDIR /app

EXPOSE 80

ENTRYPOINT ["/app/capitano", "serve", "--http", "localhost:80"]

```

## Setup Capitano and create admin account:

Before you can publish your website, you must first deploy the container. Pre-built docker images will be available soon on Github and Docker Hub. Once deployed, you need to create your main admin account using http://yourdomain.com/_/.

There are many ways you can deploy a docker container. I will give examples in future releaess of this software.

## Setup your SvelteKit project:

Create your SvelteKit app like normal. Because we'll be deploying

```sh
npm create svelte@latest my-app
cd my-app
npm install
```

Install the Node Adaptor with npm i -D @sveltejs/adapter-node, then add the adapter to your svelte.config.js:

```js
import adapter from '@sveltejs/adapter-node';

export default {
    kit: {
        adapter: adapter()
    }
};
```

# Publish your app:

Currently, you must use the `tar` and `curl` commands to publish your website. Future versions of Capitano will include a `capitano publish` command with some extra security to encrypt your token, require a publishing password, etc.

Create a publish.sh with the following contents.

```sh
#!/bin/bash

set -e

# If the .env file does not exist, then create it
if [ ! -f .env ]; then
    touch .env
fi

source .env

# If no BASE_URL is set, then ask for it
if [ -z "$BASE_URL" ]; then
    # Prompt the user for the Base URL in the correct format
    read -p "Base URL (e.g., http://yourdomain.com): " BASE_URL

    # Set the BASE_URL in the .env file
    echo "BASE_URL=$BASE_URL" >> .env
fi

# If there is an admin token, then the user has previously authenticated
if [[ -z "$ADMIN_TOKEN" ]]; then
    # Ask for the identity of the user and the password
    echo "Capitano is built using Pocketbase Framework. You can access the admin panel at $BASE_URL/_/. Once you create an admin account, you can use those credentials create a token used to publish your web app."
    
    read -p "Admin Email: " identity
    read -sp "Admin Password: " password

    # Make a request to the API to get the token
    response=$(curl -X POST -d "identity=$identity&password=$password" $BASE_URL/api/admins/auth-with-password)
    token=$(echo "$response" | grep -o '"token":"[^"]*' | cut -d':' -f2 | tr -d '"')

    # If the token is empty, then the user is not authenticated
    if [[ -z "$token" ]]; then
        echo "Authentication failed. Be sure you can sign into the admin panel with the credentials you provided, or that you correctly set the BASE_URL in the .env file."
        exit 1
    fi

    # Set the token in the .env file
    echo "ADMIN_TOKEN=$token" >> .env
fi

# Build the website
npm run build

# Create the tarball
tar -cf website.tar --transform 's,^build/,dist/,' build/ package.json package-lock.json

response=$(curl -X POST  -H "Authorization: Bearer $ADMIN_TOKEN"  -F "file=@website.tar" $BASE_URL)

echo "$response"

# Remove the tar file
rm website.tar
```

You can now run this script in WSL as-is, or you can chmod +X the script and run it in other Linux distrobutions.

You'll be prompted to specify your FQDN, i.e., https://yourdomain.com, your admin email and password. This will request and store a token which then can be used to publish your app in the future.

Run the the script once again to publish your site.

# Futher information:

Capitano is very much a work in progress. It expects your website.tar file to include the build/, package.json and packages.json files. It will perform an `npm ci --omit dev` on the server side prior to deployments, so NPM is the tool of chocie at the moment.

I will be making a number of changes to make customization possible, such as using yarn, or pnpm, specify the startup command and allow for red/green zero downtime deployments, rollbacks, etc.

I also have plans enable provisioning of Docker Swarms, Kubernetes clusters using Capitano, but that will be in the Pro version that is paid. In the pro version, Automatic SSL's are provided.