@echo off
echo Starting Overcooked Review System...
echo.
echo Make sure Docker Desktop is running!
echo.

REM Clean up any existing containers
docker-compose down

REM Build and start the services
docker-compose up --build -d

echo.
echo Services are starting up...
echo.
echo Waiting for all services to be ready...
timeout /t 30 /nobreak > nul

echo.
echo =====================================
echo Overcooked Review System is ready!
echo.
echo Frontend: http://localhost
echo Backend API: http://localhost:8085
echo Database: localhost:5444
echo Redis: localhost:6379
echo Kafka: localhost:9092
echo.
echo To stop the system, run: docker-compose down
echo =====================================
echo.

pause