@echo off
echo Running unit tests for all services...
echo.

echo [agg-svc]
go test -v -cover ./agg-svc/internal/tests/...
echo.

echo [analytics-svc]
go test -v -cover ./analytics-svc/internal/tests/...
echo.

echo [api-gateway]
go test -v -cover ./api-gateway/internal/tests/...
echo.

echo [dish-svc]
go test -v -cover ./dish-svc/internal/tests/...
echo.

echo [rate-svc]
go test -v -cover ./rate-svc/internal/tests/...
echo.

echo ============================
echo Health checks...
echo ============================
curl -s http://localhost:8080/health > nul && echo ✅ API Gateway || echo ❌ API Gateway
curl -s http://localhost:8081/health > nul && echo ✅ Dish Service || echo ❌ Dish Service
curl -s http://localhost:8082/health > nul && echo ✅ Rate Service || echo ❌ Rate Service
curl -s http://localhost:8084/health > nul && echo ✅ Analytics Service || echo ❌ Analytics Service

echo.
echo Tests completed.
pause