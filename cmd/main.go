package main

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net"
	"net/http"
	"os"
	"os/signal"
	"path"
	"path/filepath"
	"strings"
	"syscall"
	"time"

	"aeibi/api"
	"aeibi/internal/auth"
	"aeibi/internal/controller"
	"aeibi/internal/repository/db"
	"aeibi/internal/repository/oss"
	"aeibi/internal/service"

	"github.com/grpc-ecosystem/grpc-gateway/v2/runtime"
	"github.com/minio/minio-go/v7"
	"github.com/minio/minio-go/v7/pkg/credentials"
	"google.golang.org/grpc"

	_ "modernc.org/sqlite"
)

// config
const (
	grpcAddr      = ":9090"
	httpAddr      = ":8080"
	assetHTTPAddr = ":8081"

	sqliteDSN     = "file:aeibi.db?_pragma=busy_timeout(5000)&cache=shared"
	migrationsDir = "internal/repository/db/migration"

	ossEndpoint  = ""
	ossAccessKey = ""
	ossSecretKey = ""
	ossBucket    = "aeibi"
	ossUseSSL    = false

	jwtSecret  = ""
	jwtIssuer  = ""
	jwtTTL     = time.Second * 200
	refreshTTL = 30 * 24 * time.Hour
)

func main() {
	slog.SetDefault(slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{
		Level: slog.LevelInfo,
	})))

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	dbConn, err := initDB(ctx)
	if err != nil {
		slog.Error("init db", "error", err)
		os.Exit(1)
	}
	defer dbConn.Close()

	ossClient, err := initOSS(ctx)
	if err != nil {
		slog.Error("init oss", "error", err)
		os.Exit(1)
	}

	userSvc := service.NewUserService(dbConn, ossClient, jwtSecret, jwtIssuer, jwtTTL, refreshTTL)
	userHandler := controller.NewUserHandler(userSvc)

	postSvc := service.NewPostService(dbConn, ossClient)
	postHandler := controller.NewPostHandler(postSvc)

	grpcServer := grpc.NewServer(
		grpc.UnaryInterceptor(auth.NewAuthUnaryServerInterceptor(jwtSecret)),
	)
	api.RegisterUserServiceServer(grpcServer, userHandler)
	api.RegisterPostServiceServer(grpcServer, postHandler)

	lis, err := net.Listen("tcp", grpcAddr)
	if err != nil {
		slog.Error("listen gRPC", "error", err)
		os.Exit(1)
	}
	go func() {
		if err := grpcServer.Serve(lis); err != nil && !errors.Is(err, grpc.ErrServerStopped) {
			slog.Error("gRPC server", "error", err)
			os.Exit(1)
		}
	}()

	mux := runtime.NewServeMux(
		runtime.WithMetadata(auth.GatewayMetadataExtractor),
	)
	if err := api.RegisterUserServiceHandlerServer(ctx, mux, userHandler); err != nil {
		slog.Error("register HTTP gateway", "error", err)
		os.Exit(1)
	}
	httpServer := &http.Server{
		Addr:    httpAddr,
		Handler: mux,
	}
	assetServer := &http.Server{
		Addr:    assetHTTPAddr,
		Handler: assetProxyHandler(ossClient),
	}

	go func() {
		<-ctx.Done()
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()

		grpcServer.GracefulStop()
		if err := httpServer.Shutdown(shutdownCtx); err != nil {
			slog.Warn("HTTP shutdown", "error", err)
		}
		if err := assetServer.Shutdown(shutdownCtx); err != nil {
			slog.Warn("asset HTTP shutdown", "error", err)
		}
	}()

	slog.Info("gRPC server listening", "addr", grpcAddr)
	slog.Info("HTTP gateway listening", "addr", httpAddr)
	go func() {
		if err := assetServer.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			slog.Error("asset HTTP server", "error", err)
			os.Exit(1)
		}
	}()
	slog.Info("Asset server listening", "addr", assetHTTPAddr)

	if err := httpServer.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
		slog.Error("HTTP server", "error", err)
		os.Exit(1)
	}
}

func initDB(ctx context.Context) (*sql.DB, error) {
	if err := runMigrations(); err != nil {
		return nil, err
	}

	dbConn, err := sql.Open("sqlite", sqliteDSN)
	if err != nil {
		return nil, fmt.Errorf("open sqlite: %w", err)
	}

	dbConn.SetMaxOpenConns(1)
	dbConn.SetMaxIdleConns(1)

	if err := dbConn.PingContext(ctx); err != nil {
		dbConn.Close()
		return nil, fmt.Errorf("ping sqlite: %w", err)
	}

	return dbConn, nil
}

func runMigrations() error {
	migrationPath, err := filepath.Abs(migrationsDir)
	if err != nil {
		return fmt.Errorf("resolve migrations dir: %w", err)
	}

	migrationDSN := strings.TrimPrefix(sqliteDSN, "file:")
	if err := db.Migration(fmt.Sprintf("file://%s", migrationPath), fmt.Sprintf("sqlite://%s", migrationDSN)); err != nil {
		return fmt.Errorf("run migrations: %w", err)
	}
	return nil
}

func initOSS(ctx context.Context) (*oss.OSS, error) {
	client, err := minio.New(ossEndpoint, &minio.Options{
		Creds:  credentials.NewStaticV4(ossAccessKey, ossSecretKey, ""),
		Secure: ossUseSSL,
	})
	if err != nil {
		return nil, fmt.Errorf("init minio client: %w", err)
	}

	bucketCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	exists, err := client.BucketExists(bucketCtx, ossBucket)
	if err != nil {
		return nil, fmt.Errorf("check bucket %q: %w", ossBucket, err)
	}
	if !exists {
		if err := client.MakeBucket(bucketCtx, ossBucket, minio.MakeBucketOptions{}); err != nil {
			return nil, fmt.Errorf("create bucket %q: %w", ossBucket, err)
		}
	}

	return oss.New(client, ossBucket), nil
}

func assetProxyHandler(ossClient *oss.OSS) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet && r.Method != http.MethodHead {
			w.WriteHeader(http.StatusMethodNotAllowed)
			return
		}

		objectPath := path.Clean("/" + strings.TrimPrefix(r.URL.Path, "/"))
		objectPath = strings.TrimPrefix(objectPath, "/")
		objectPath = strings.TrimPrefix(objectPath, "assets/")
		if objectPath == "" || strings.HasPrefix(objectPath, "..") {
			http.Error(w, "invalid asset path", http.StatusBadRequest)
			return
		}

		reader, info, err := ossClient.GetObject(r.Context(), objectPath)
		if err != nil {
			if errors.Is(err, oss.ErrObjectNotFound) {
				http.NotFound(w, r)
				return
			}
			slog.Error("fetch asset", "path", objectPath, "error", err)
			http.Error(w, "failed to fetch asset", http.StatusInternalServerError)
			return
		}
		defer reader.Close()

		if info.ContentType != "" {
			w.Header().Set("Content-Type", info.ContentType)
		}
		if r.Method == http.MethodHead {
			return
		}
		if _, err := io.Copy(w, reader); err != nil {
			slog.Warn("stream asset", "path", objectPath, "error", err)
		}
	})
}
