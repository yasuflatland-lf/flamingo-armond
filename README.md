# Framingo Armond
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
  
# Run Locally
```
make server
```