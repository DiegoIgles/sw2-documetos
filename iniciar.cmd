@echo off
echo ============================================
echo   MICROSERVICIO DE DOCUMENTOS - INICIO
echo ============================================
echo.

echo [1/3] Construyendo imagen Docker...
docker-compose build

echo.
echo [2/3] Levantando servicios (MongoDB + Documentos)...
docker-compose up -d

echo.
echo [3/3] Esperando que el servicio inicie...
timeout /t 5 /nobreak > nul

echo.
echo ============================================
echo   Verificando estado del servicio...
echo ============================================
curl -s http://localhost:8081/health
echo.

echo.
echo ============================================
echo   MICROSERVICIO INICIADO CORRECTAMENTE
echo ============================================
echo.
echo   URL: http://localhost:8081
echo   MongoDB: mongodb://localhost:27017
echo   Health Check: http://localhost:8081/health
echo.
echo   Para ver logs: docker-compose logs -f
echo   Para detener: docker-compose down
echo ============================================
pause
