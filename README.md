# Flamingo Armond
🔥Tinder like Flashcard App. 

![flamingo-armond backend](https://github.com/yasuflatland-lf/flamingo-armond/actions/workflows/backend.yml/badge.svg)
![flamingo-armond frontend](https://github.com/yasuflatland-lf/flamingo-armond/actions/workflows/frontend.yml/badge.svg)

# Architecture
- **Render** (Static site for React, GraphQL service)
- **Superbase** (Database and authentication)
  
![Architecture](./docs/diagram.png)

# Environment
This repository is structured as a mono repository. 

| Directroy | Discription |
|:--|:--|
|frontend | Frontend implementation |
|backend| Backend GraphQL API server |

## frontend
- Typescript
- React
- Vite

## backend
- Go
- Echo
- gqlgen (GraphQL)
  
# How to Deploy to Production

## Set up Services on Render
The `render.yaml` is where all configurations are gathered and assosiated with this repository. As soon as a new commit is added to `master` branch, the depolyment is triggered.

## Set up database on Superbase
`Flamingo Armond` uses [Superbase](https://supabase.com/) for the [Database (Postgres)](https://supabase.com/database) and [Auth](https://supabase.com/auth). All configurations and environment valuables are configured on the dashboard. Grab configurations from `.env` file and apply them here, such as database name, user name, user password, SSL enablement, e.g.

### Database Settings
Database setting for user, password, go `Settings -> Configuration -> Database` on the `Superbase` console.

## Set up Auth on Superbase
### Set up OAuth API on Google Cloud
Create `Client ID` and `Client Secret` on `Google`

Refer [this document](https://support.google.com/workspacemigrate/answer/9222992?hl=ja) for the details of set up.

#### Check scope.
Please see [this page](https://supabase.com/docs/guides/auth/social-login/auth-google?queryGroups=platform&platform=web&queryGroups=environment&environment=client&queryGroups=framework&framework=sveltekit#application-code-configuration) for the OAuth scope to be configured for `Superbase`

### Set up Auth on Superbase
1. Navigate to the dashboard on `Superbase` and chose `Google Auth`
2. Fill out `Client ID` , `Client Secret` and `Authorized Client IDs`

# Run Locally
```
make server
```
