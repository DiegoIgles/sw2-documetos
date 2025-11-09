# 🗂 Microservicio de Documentos

Servicio backend en **Go** para gestión de documentos en un sistema legal.  
Proporciona endpoints para **subida, descarga, listado y eliminación de documentos**, usando **MongoDB** y **GridFS**.

---

## 🚀 Requisitos

- **Docker y Docker Compose** (versión 20.10+)
- **Go 1.22+** (solo para desarrollo local sin Docker)

---

## ⚙️ Instalación y Ejecución

### Con Docker (Recomendado)

```bash
# 1. Configurar variables de entorno
# Editar .env y verificar JWT_SECRET

# 2. Levantar servicios (MongoDB + API)
docker-compose up -d

# 3. Verificar que esté corriendo
curl http://localhost:8081/health
```

En Windows también puedes usar el script incluido `iniciar.cmd` (hace build y levanta los servicios y ejecuta un health-check):

```powershell
.\iniciar.cmd
```

Para detener los servicios en Windows hay un script `detener.cmd` que ejecuta `docker-compose down`:

```powershell
.\detener.cmd
```

### Desarrollo Local (sin Docker)

```bash
# 1. Instalar dependencias
go mod download
go mod tidy

# 2. Levantar MongoDB
docker run -d -p 27017:27017 --name mongo mongo:6

# 3. Configurar .env
# Cambiar MONGO_URI=mongodb://localhost:27017

# 4. Ejecutar
go run main.go
```

---

## 🔧 Configuración (.env)

```env
PORT=8081
MONGO_URI=mongodb://mongo:27017          # Para Docker
JWT_SECRET=cambia_este_secreto_en_env    # DEBE coincidir con backend NestJS
MAX_UPLOAD_MB=50
ALLOWED_ORIGINS=*
```

> ⚠️ **IMPORTANTE**: El `JWT_SECRET` debe ser **exactamente el mismo** que usa el microservicio backend (NestJS) para que la autenticación funcione.

---

## 📡 Endpoints

### Públicos (sin autenticación)

#### GET /health
Health check del servicio
```bash
curl http://localhost:8081/health
# Response: {"status":"ok"}
```

Listar todos los documentos (con paginación opcional)
```bash
curl "http://localhost:8081/admin/documentos?limit=10&offset=0"
```

```bash
curl http://localhost:8081/documentos/507f1f77bcf86cd799439011 --output documento.pdf
### Protegidos (requieren JWT)

Incluir header: `Authorization: Bearer <token>`

#### POST /documentos
```bash
curl -X POST http://localhost:8081/documentos \
  -H "Authorization: Bearer <tu_token>" \
  -F "file=@documento.pdf" \
```

**Response:**
```json
  "doc_id": "507f1f77bcf86cd799439011",
  "filename": "documento.pdf",
  "id_expediente": 1
}
```
#### GET /mis-documentos
```bash
curl http://localhost:8081/mis-documentos \
  -H "Authorization: Bearer <tu_token>"
```

#### GET /expedientes/:id_expediente/documentos
Listar documentos de un expediente específico
```bash
curl http://localhost:8081/expedientes/1/documentos \
  -H "Authorization: Bearer <tu_token>"
```

#### DELETE /documentos/:doc_id
Eliminar un documento
```bash
curl -X DELETE http://localhost:8081/documentos/507f1f77bcf86cd799439011 \
  -H "Authorization: Bearer <tu_token>"
```

---

## 🔐 Autenticación JWT

El servicio valida tokens JWT con la siguiente estructura:

```json
{
  "sub": 123,           // id_cliente
  "tipo": "CLIENTE",    // "CLIENTE" | "ADMIN" | "OPERADOR"
  "exp": 1699999999
}
```

Los tokens deben ser generados por el microservicio de autenticación principal usando el mismo `JWT_SECRET`.

---

## 📦 Estructura de Datos

### Documento (MongoDB)
```json
{
  "_id": "ObjectId",
  "doc_id": "ObjectId",          // ID en GridFS
  "filename": "documento.pdf",
  "size": 125847,
  "id_cliente": 1,
  "id_expediente": 5,
  "created_at": "2024-11-08T10:30:00Z"
}
```

---

## 🐳 Docker

### Servicios incluidos:
- **mongo**: MongoDB 6 (puerto 27017)
- **documentos**: API Go (puerto 8081)

### Comandos útiles:

```bash
# Ver logs
docker-compose logs -f

# Reiniciar
docker-compose restart

# Detener
docker-compose down

# Detener y eliminar volúmenes (CUIDADO: borra datos)
docker-compose down -v

# Reconstruir
docker-compose up --build -d
```

---

## 📚 Documentación (OpenAPI / Swagger UI)

Se añadió una especificación OpenAPI (`openapi.json`) y una página Swagger UI para probar los endpoints desde el navegador.

- Especificación: `GET /openapi.json`
- UI interactiva: `GET /docs` (Swagger UI cargado desde CDN)

Notas importantes:
- El archivo `openapi.json` se sirve como recurso estático y se incluyó en la imagen Docker (ver `Dockerfile`).
- Si estás usando Docker (imagen construida anteriormente), debes reconstruir la imagen para que el archivo esté disponible dentro del contenedor:

```bash
docker-compose up --build -d
```

- Verifica la spec con:
```bash
curl http://localhost:8081/openapi.json
```

- Abre la UI en tu navegador:
```
http://localhost:8081/docs
```

- Autorización: los endpoints protegidos requieren un header `Authorization: Bearer <token>`.
  - En Swagger UI usa el botón "Authorize" y pega `Bearer <tu_token>` (si pega solo el token y no funciona, prueba incluyendo el prefijo `Bearer `).

- Desarrollo rápido: si no quieres reconstruir la imagen constantemente durante desarrollo, puedes:
  - Ejecutar la app localmente con `go run main.go` (asegúrate de que `MONGO_URI` apunte a tu Mongo local), o
  - Modificar `docker-compose.yml` para montar el proyecto como volumen en el contenedor (permitirá ver cambios sin rebuild).

- Producción: si no quieres exponer la documentación en producción, puedes controlar su exposición mediante una variable de entorno (por ejemplo `SERVE_DOCS`) y registrar las rutas `/openapi.json` y `/docs` solo en entornos de desarrollo.


## 🔍 Troubleshooting

### Error: "Connection refused" a MongoDB

**Causa**: MongoDB no está corriendo o la URL es incorrecta

**Solución**:
```bash
# Verificar que MongoDB esté corriendo
docker ps | grep mongo

# Si no está, levantar servicios
docker-compose up -d
```

### Error 401 Unauthorized

**Causa**: Token inválido o `JWT_SECRET` no coincide

**Solución**:
1. Verificar que el `JWT_SECRET` en `.env` sea el mismo que el backend
2. Obtener un token fresco del endpoint de login
3. Incluir el header: `Authorization: Bearer <token>`

### Error: "File too large"

**Causa**: Archivo excede el límite de `MAX_UPLOAD_MB`

**Solución**: Aumentar `MAX_UPLOAD_MB` en `.env` (por defecto: 50MB)

---

## 🧪 Pruebas

### 1. Health Check
```bash
curl http://localhost:8081/health
```

### 2. Subir un documento de prueba
```bash
# Primero obtener token del servicio de auth
TOKEN="<tu_token_aqui>"

# Subir archivo
curl -X POST http://localhost:8081/documentos \
  -H "Authorization: Bearer $TOKEN" \
  -F "file=@test.pdf" \
  -F "id_expediente=1"
```

### 3. Listar documentos
```bash
curl http://localhost:8081/mis-documentos \
  -H "Authorization: Bearer $TOKEN"
```

---

## 📊 Integración con API Gateway

Este microservicio está diseñado para funcionar con el API Gateway GraphQL:

- **Gateway**: http://localhost:8000/graphql
- **Documentos**: http://localhost:8081

El gateway expone queries/mutations GraphQL que internamente llaman a este servicio REST.

**Ver**: Documentación del API Gateway para ejemplos de uso con GraphQL

---

## 🛠️ Stack Tecnológico

- **Lenguaje**: Go 1.22
- **Framework**: Gin (HTTP router)
- **Base de datos**: MongoDB 6
- **Almacenamiento**: GridFS
- **Autenticación**: JWT (golang-jwt/jwt)
- **Contenedores**: Docker + Docker Compose

---

## 📝 Notas Importantes

1. **GridFS**: Los archivos se almacenan en GridFS (no en el filesystem)
2. **Seguridad**: Los clientes solo pueden ver/eliminar sus propios documentos
3. **CORS**: Por defecto permite todos los orígenes (`*`). Cambiar en producción.
4. **Límite de tamaño**: Por defecto 50MB por archivo (configurable)
5. **JWT Secret**: **DEBE** ser el mismo en todos los microservicios

---

## 🚀 Listo para Producción

Para producción, recuerda:
- ✅ Cambiar `JWT_SECRET` a un valor seguro
- ✅ Configurar `ALLOWED_ORIGINS` con dominios específicos
- ✅ Usar MongoDB con autenticación
- ✅ Configurar backups de MongoDB
- ✅ Aumentar recursos si hay muchos uploads concurrentes
- ✅ Monitorear espacio en disco de MongoDB

---

**¡Microservicio listo para usar!** 🎉