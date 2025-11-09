@echo off
echo ============================================
echo   DETENIENDO MICROSERVICIO DE DOCUMENTOS
echo ============================================
echo.

docker-compose down

echo.
echo ============================================
echo   SERVICIOS DETENIDOS
echo ============================================
echo.
echo Para limpiar datos (MongoDB):
echo   docker-compose down -v
echo ============================================
pause
