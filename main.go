package main

import (
	"context"
	"io"
	"log"
	"net/http"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/golang-jwt/jwt/v5"
	"github.com/joho/godotenv"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/gridfs"
	"go.mongodb.org/mongo-driver/mongo/options"
)

type Claims struct {
	Sub  int64  `json:"sub"`  // id_cliente (para clientes)
	Tipo string `json:"tipo"` // "CLIENTE" | "ADMIN" | "OPERADOR"
	jwt.RegisteredClaims
}

type DocMeta struct {
	ID           primitive.ObjectID `bson:"_id,omitempty" json:"_id"`
	DocID        primitive.ObjectID `bson:"doc_id"        json:"-"`
	DocIDHex     string             `bson:"-"            json:"doc_id"`
	Filename     string             `bson:"filename"     json:"filename"`
	Size         int64              `bson:"size"         json:"size"`
	IDCliente    int64              `bson:"id_cliente"   json:"id_cliente"`
	IDExpediente int64              `bson:"id_expediente" json:"id_expediente"`
	CreatedAt    time.Time          `bson:"created_at"   json:"created_at"`
}

func mustEnv(k string) string {
	v := os.Getenv(k)
	if v == "" {
		log.Fatalf("env %s requerido", k)
	}
	return v
}

func getEnv(k, def string) string {
	if v := os.Getenv(k); v != "" {
		return v
	}
	return def
}

func int64FromEnv(k string, def int64) int64 {
	if v := os.Getenv(k); v != "" {
		if n, err := strconv.ParseInt(v, 10, 64); err == nil {
			return n
		}
	}
	return def
}

func main() {
	_ = godotenv.Load()

	port := getEnv("PORT", "8081")
	mongoURI := getEnv("MONGO_URI", "mongodb://localhost:27017")
	mongoDB := getEnv("MONGO_DB", "documentos_db")
	jwtSecret := mustEnv("JWT_SECRET")
	maxUploadMB := int64FromEnv("MAX_UPLOAD_MB", 50)
	allowed := getEnv("ALLOWED_ORIGINS", "*")

	// Conexión a Mongo
	client, err := mongo.Connect(context.Background(), options.Client().ApplyURI(mongoURI))
	if err != nil {
		log.Fatal("mongo connect:", err)
	}
	defer client.Disconnect(context.Background())

	db := client.Database(mongoDB)
	bucket, err := gridfs.NewBucket(db)
	if err != nil {
		log.Fatal("gridfs bucket:", err)
	}

	// Colección de metadatos
	coll := db.Collection("documentos")
	// Índices (tipo explícito para evitar "missing type in composite literal")
	_, _ = coll.Indexes().CreateMany(context.Background(), []mongo.IndexModel{
		mongo.IndexModel{
			Keys: bson.D{{Key: "id_cliente", Value: 1}, {Key: "created_at", Value: -1}},
		},
		mongo.IndexModel{
			Keys: bson.D{{Key: "id_expediente", Value: 1}, {Key: "created_at", Value: -1}},
		},
		mongo.IndexModel{
			Keys:    bson.D{{Key: "doc_id", Value: 1}},
			Options: options.Index().SetUnique(true),
		},
	})

	r := gin.Default()

	// CORS simple
	r.Use(func(c *gin.Context) {
		c.Header("Access-Control-Allow-Origin", allowed)
		c.Header("Access-Control-Allow-Headers", "Authorization, Content-Type")
		c.Header("Access-Control-Allow-Methods", "GET, POST, DELETE, OPTIONS")
		if c.Request.Method == "OPTIONS" {
			c.AbortWithStatus(204)
			return
		}
		c.Next()
	})

	// =========================
	//  ENDPOINT PÚBLICO (ADMIN)
	// =========================
	// GET /admin/documentos  (SIN token)  -> lista todos con paginación opcional
	r.GET("/admin/documentos", func(c *gin.Context) {
		limStr := c.Query("limit")
		offStr := c.Query("offset")

		var lim, off int64
		if limStr != "" {
			if v, err := strconv.ParseInt(limStr, 10, 64); err == nil && v > 0 && v <= 200 {
				lim = v
			}
		}
		if offStr != "" {
			if v, err := strconv.ParseInt(offStr, 10, 64); err == nil && v >= 0 {
				off = v
			}
		}

		findOpts := options.Find().SetSort(bson.D{{Key: "created_at", Value: -1}})
		if lim > 0 {
			findOpts.SetLimit(lim)
		}
		if off > 0 {
			findOpts.SetSkip(off)
		}

		cur, err := coll.Find(c.Request.Context(), bson.M{}, findOpts)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "no se pudo listar"})
			return
		}
		defer cur.Close(c.Request.Context())

		var out []DocMeta
		for cur.Next(c.Request.Context()) {
			var d DocMeta
			if err := cur.Decode(&d); err != nil {
				continue
			}
			d.DocIDHex = d.DocID.Hex()
			out = append(out, d)
		}
		if err := cur.Err(); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "error en cursor"})
			return
		}

		c.JSON(http.StatusOK, out)
	})

	// Guard JWT: valida token y coloca claims en contexto
	auth := func(c *gin.Context) {
		h := c.GetHeader("Authorization")
		if !strings.HasPrefix(h, "Bearer ") {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "token requerido"})
			return
		}
		tokenStr := strings.TrimPrefix(h, "Bearer ")
		token, err := jwt.ParseWithClaims(tokenStr, &Claims{}, func(t *jwt.Token) (interface{}, error) {
			return []byte(jwtSecret), nil
		})
		if err != nil || !token.Valid {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "token inválido"})
			return
		}
		claims := token.Claims.(*Claims)
		c.Set("id_cliente", claims.Sub)
		c.Set("tipo", claims.Tipo)
		c.Next()
	}

	api := r.Group("/")
	api.Use(auth)

	// POST /documentos  (multipart: file, id_expediente)
	api.POST("/documentos", func(c *gin.Context) {
		c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, maxUploadMB*1024*1024)

		f, header, err := c.Request.FormFile("file")
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "file es requerido"})
			return
		}
		defer f.Close()

		idExpStr := c.PostForm("id_expediente")
		if idExpStr == "" {
			c.JSON(http.StatusBadRequest, gin.H{"error": "id_expediente es requerido"})
			return
		}
		idExp, err := strconv.ParseInt(idExpStr, 10, 64)
		if err != nil || idExp <= 0 {
			c.JSON(http.StatusBadRequest, gin.H{"error": "id_expediente inválido"})
			return
		}

		idClienteAny, _ := c.Get("id_cliente")
		idCliente := idClienteAny.(int64)

		uploadStream, err := bucket.OpenUploadStream(header.Filename)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "no se pudo abrir stream"})
			return
		}
		defer uploadStream.Close()

		if _, err := io.Copy(uploadStream, f); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "falló escritura"})
			return
		}

		oid := uploadStream.FileID.(primitive.ObjectID)

		// Guardar metadatos
		doc := DocMeta{
			DocID:        oid,
			Filename:     header.Filename,
			Size:         header.Size,
			IDCliente:    idCliente,
			IDExpediente: idExp,
			CreatedAt:    time.Now(),
		}
		if _, err := coll.InsertOne(c.Request.Context(), doc); err != nil {
			// rollback best-effort en gridfs si falla metadata
			_ = bucket.Delete(oid)
			c.JSON(http.StatusInternalServerError, gin.H{"error": "no se pudo guardar metadatos"})
			return
		}

		c.JSON(http.StatusOK, gin.H{
			"doc_id":        oid.Hex(),
			"filename":      header.Filename,
			"size":          header.Size,
			"id_expediente": idExp,
		})
	})

	// GET /mis-documentos  -> documentos del cliente autenticado
	api.GET("/mis-documentos", func(c *gin.Context) {
		idClienteAny, _ := c.Get("id_cliente")
		idCliente := idClienteAny.(int64)

		cur, err := coll.Find(c.Request.Context(),
			bson.M{"id_cliente": idCliente},
			options.Find().SetSort(bson.D{{Key: "created_at", Value: -1}}),
		)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "no se pudo listar"})
			return
		}
		defer cur.Close(c.Request.Context())

		var out []DocMeta
		for cur.Next(c.Request.Context()) {
			var d DocMeta
			if err := cur.Decode(&d); err != nil {
				continue
			}
			d.DocIDHex = d.DocID.Hex()
			out = append(out, d)
		}
		if err := cur.Err(); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "error en cursor"})
			return
		}

		c.JSON(http.StatusOK, out)
	})

	// GET /expedientes/:id_expediente/documentos  -> por expediente
	api.GET("/expedientes/:id_expediente/documentos", func(c *gin.Context) {
		idExpStr := c.Param("id_expediente")
		idExp, err := strconv.ParseInt(idExpStr, 10, 64)
		if err != nil || idExp <= 0 {
			c.JSON(http.StatusBadRequest, gin.H{"error": "id_expediente inválido"})
			return
		}

		tipoAny, _ := c.Get("tipo")
		tipo := tipoAny.(string)

		filter := bson.M{"id_expediente": idExp}

		// Si es CLIENTE, restringimos a sus propios documentos (defensa básica)
		if tipo == "CLIENTE" {
			idClienteAny, _ := c.Get("id_cliente")
			filter["id_cliente"] = idClienteAny.(int64)
		}

		cur, err := coll.Find(c.Request.Context(),
			filter,
			options.Find().SetSort(bson.D{{Key: "created_at", Value: -1}}),
		)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "no se pudo listar"})
			return
		}
		defer cur.Close(c.Request.Context())

		var out []DocMeta
		for cur.Next(c.Request.Context()) {
			var d DocMeta
			if err := cur.Decode(&d); err != nil {
				continue
			}
			d.DocIDHex = d.DocID.Hex()
			out = append(out, d)
		}
		if err := cur.Err(); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "error en cursor"})
			return
		}

		c.JSON(http.StatusOK, out)
	})

	// GET /documentos/:doc_id  (stream)
	// --- DESCARGA PÚBLICA ---
	r.GET("/documentos/:doc_id", func(c *gin.Context) {
		id := c.Param("doc_id")
		oid, err := primitive.ObjectIDFromHex(id)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "doc_id inválido"})
			return
		}

		c.Header("Content-Type", "application/pdf")
		c.Header("Content-Disposition", "attachment; filename="+id+".pdf")

		if _, err := bucket.DownloadToStream(oid, c.Writer); err != nil {
			c.JSON(http.StatusNotFound, gin.H{"error": "no encontrado"})
			return
		}
	})
	// --- FIN DESCARGA PÚBLICA ---

	// DELETE /documentos/:doc_id
	api.DELETE("/documentos/:doc_id", func(c *gin.Context) {
		id := c.Param("doc_id")
		oid, err := primitive.ObjectIDFromHex(id)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "doc_id inválido"})
			return
		}

		// Seguridad: si es CLIENTE, el doc debe ser suyo
		if tipoAny, ok := c.Get("tipo"); ok && tipoAny.(string) == "CLIENTE" {
			idClienteAny, _ := c.Get("id_cliente")
			ownCount, _ := coll.CountDocuments(c.Request.Context(), bson.M{"doc_id": oid, "id_cliente": idClienteAny.(int64)})
			if ownCount == 0 {
				c.JSON(http.StatusForbidden, gin.H{"error": "no autorizado"})
				return
			}
		}

		// Borra en gridfs y metadatos
		if err := bucket.Delete(oid); err != nil {
			c.JSON(http.StatusNotFound, gin.H{"error": "no encontrado"})
			return
		}
		_, _ = coll.DeleteOne(c.Request.Context(), bson.M{"doc_id": oid})
		c.Status(http.StatusNoContent)
	})

	// Health
	r.GET("/health", func(c *gin.Context) { c.JSON(200, gin.H{"status": "ok"}) })

	// Servir especificación OpenAPI y Swagger UI
	r.GET("/openapi.json", func(c *gin.Context) {
		// Servir el archivo estático openapi.json ubicado en la raíz del proyecto
		c.File("./openapi.json")
	})

	r.GET("/docs", func(c *gin.Context) {
		html := `<!DOCTYPE html>
<html lang="es">
	<head>
		<meta charset="utf-8" />
		<meta name="viewport" content="width=device-width, initial-scale=1" />
		<title>Docs - Microservicio Documentos</title>
		<link rel="stylesheet" href="https://unpkg.com/swagger-ui-dist@4/swagger-ui.css" />
		<style>body { margin:0; padding:0; }</style>
	</head>
	<body>
		<div id="swagger-ui"></div>
		<script src="https://unpkg.com/swagger-ui-dist@4/swagger-ui-bundle.js"></script>
		<script src="https://unpkg.com/swagger-ui-dist@4/swagger-ui-standalone-preset.js"></script>
		<script>
			window.onload = function() {
				const ui = SwaggerUIBundle({
					url: '/openapi.json',
					dom_id: '#swagger-ui',
					presets: [SwaggerUIBundle.presets.apis, SwaggerUIStandalonePreset],
					layout: "BaseLayout",
					deepLinking: true
				})
				window.ui = ui
			}
		</script>
	</body>
</html>`

		c.Data(http.StatusOK, "text/html; charset=utf-8", []byte(html))
	})

	log.Println("Documentos service escuchando en :" + port)
	s := &http.Server{
		Addr:           ":" + port,
		Handler:        r,
		ReadTimeout:    20 * time.Second,
		WriteTimeout:   120 * time.Second,
		MaxHeaderBytes: 1 << 20,
	}
	log.Fatal(s.ListenAndServe())
}
