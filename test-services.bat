@echo off
echo Testing Overcooked Review System Services...
echo.

echo ============================
echo Running Go unit tests...
echo ============================
echo.

echo [agg-svc]
pushd agg-svc >nul
go test ./...
popd
echo.

echo [analytics-svc]
pushd analytics-svc >nul
go test ./...
popd
echo.

echo [api-gateway]
pushd api-gateway >nul
go test ./...
popd
echo.

echo [dish-svc]
pushd dish-svc >nul
go test ./...
popd
echo.

echo [rate-svc]
pushd rate-svc >nul
go test ./...
popd
echo.

echo ============================
echo Unit tests finished.
echo ============================
echo.

REM Test health endpoints
echo Testing API Gateway...
curl -s http://localhost:8080/health > nul
if %errorlevel% neq 0 (
    echo ❌ API Gateway is not responding
) else (
    echo ✅ API Gateway is healthy
)

echo.
echo Testing Dish Service...
curl -s http://localhost:8081/api/restaurants > nul
if %errorlevel% neq 0 (
    echo ❌ Dish Service is not responding
) else (
    echo ✅ Dish Service is healthy
)

echo.
echo Testing Rate Service...
curl -s http://localhost:8082/api/restaurants > nul
if %errorlevel% neq 0 (
    echo ❌ Rate Service is not responding
) else (
    echo ✅ Rate Service is healthy
)

echo.
echo Testing Analytics Service...
curl -s http://localhost:8084/health > nul
if %errorlevel% neq 0 (
    echo ❌ Analytics Service is not responding
) else (
    echo ✅ Analytics Service is healthy
)

echo.
echo Testing Frontend...
curl -s http://localhost/ > nul
if %errorlevel% neq 0 (
    echo ❌ Frontend is not responding
) else (
    echo ✅ Frontend is healthy
)

echo.
echo.
echo ============================
echo Running integration tests...
echo ============================
cd tests
go test -v -timeout=30s
cd ..
echo Service testing complete!
echo.
echo.
pause