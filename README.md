# 🗂 Documentos Service

Servicio backend en **Go** para gestión de documentos en un sistema legal.  
Proporciona endpoints para **subida, descarga, listado y eliminación de documentos**, usando **MongoDB** y **GridFS**.

---

## 🚀 Requisitos

- [Docker y Docker Compose](https://www.docker.com/)
- (Opcional para desarrollo local) [Go 1.22+](https://golang.org/)

---

## ⚙️ Instalación y ejecución con Docker

1. **Clonar el repositorio**

```bash
git clone <URL_DEL_REPOSITORIO>
cd <REPO_DOCUMENTOS>


PORT=8081
MONGO_URI=mongodb://mongo:27017
MONGO_DB=documentos_db
JWT_SECRET=cL9s8Dk3JfX2b7NqYz5vWmR1TgHqPnVx4yZsE0aB6C7D8E9F0G1H2I3J4K5L6M7N8
MAX_UPLOAD_MB=50
ALLOWED_ORIGINS=*
 
 JWT igual a la de nest

levantar docker 
docker-compose up -d
