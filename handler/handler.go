package handler

import (
	"errors"
	"io"
	"net/http"
	"strconv"
	"strings"

	"github.com/air-gases/cacheman"
	"github.com/aofei/air"
	"github.com/goproxy/goproxy.cn/base"
	"github.com/goproxy/goproxy/cacher"
	"github.com/robfig/cron/v3"
	"github.com/tidwall/gjson"
)

var (
	// qiniuViper is used to get the configuration items of the Qiniu Cloud.
	qiniuViper = base.Viper.Sub("qiniu")

	// qiniuAccessKey is the access key for the Qiniu Cloud.
	qiniuAccessKey = qiniuViper.GetString("access_key")

	// qiniuSecretKey is the secret key for the Qiniu Cloud.
	qiniuSecretKey = qiniuViper.GetString("secret_key")

	// qiniuKodoCacher is the kodo cacher for the Qiniu Cloud.
	qiniuKodoCacher = &cacher.Kodo{
		Endpoint:   qiniuViper.GetString("kodo_endpoint"),
		AccessKey:  qiniuAccessKey,
		SecretKey:  qiniuSecretKey,
		BucketName: qiniuViper.GetString("kodo_bucket_name"),
	}

	// getHeadMethods is an array contains the GET and HEAD methods.
	getHeadMethods = []string{http.MethodGet, http.MethodHead}

	// hourlyCachemanGas is used to manage the Cache-Control header.
	hourlyCachemanGas = cacheman.Gas(cacheman.GasConfig{
		Public:  true,
		MaxAge:  3600,
		SMaxAge: -1,
	})

	// moduleVersionCount is the module version count.
	moduleVersionCount int64
)

func init() {
	if err := updateModuleVersionsCount(); err != nil {
		base.Logger.Fatal().Err(err).
			Msg("failed to initialize module version count")
	}

	if _, err := base.Cron.AddJob(
		"*/10 * * * *", // Every 10 minutes
		cron.NewChain(
			cron.SkipIfStillRunning(cron.DiscardLogger),
		).Then(cron.FuncJob(func() {
			err := updateModuleVersionsCount()
			if err == nil {
				return
			}

			base.Logger.Error().Err(err).
				Msg("failed to update module version count")
		})),
	); err != nil {
		base.Logger.Fatal().Err(err).
			Msg("failed to add module version count update cron " +
				"job")
	}

	base.Air.FILE("/robots.txt", "robots.txt")
	base.Air.FILE("/favicon.ico", "favicon.ico", hourlyCachemanGas)
	base.Air.FILE(
		"/apple-touch-icon.png",
		"apple-touch-icon.png",
		hourlyCachemanGas,
	)

	base.Air.FILES("/assets", base.Air.CofferAssetRoot, hourlyCachemanGas)

	base.Air.BATCH(getHeadMethods, "/", hIndexPage)
}

// NotFound returns not found error.
func NotFound(req *air.Request, res *air.Response) error {
	res.Status = http.StatusNotFound
	return errors.New(strings.ToLower(http.StatusText(res.Status)))
}

// MethodNotAllowed returns method not allowed error.
func MethodNotAllowed(req *air.Request, res *air.Response) error {
	res.Status = http.StatusMethodNotAllowed
	return errors.New(strings.ToLower(http.StatusText(res.Status)))
}

// Error handles errors.
func Error(err error, req *air.Request, res *air.Response) {
	if res.Written {
		return
	}

	if !req.Air.DebugMode && res.Status == http.StatusInternalServerError {
		res.WriteString(strings.ToLower(http.StatusText(res.Status)))
	} else {
		res.WriteString(err.Error())
	}
}

// hIndexPage handles requests to get index page.
func hIndexPage(req *air.Request, res *air.Response) error {
	return res.Render(map[string]interface{}{
		"IsIndexPage": true,
		"ModuleVersionCount": thousandsCommaSeperated(
			moduleVersionCount,
		),
	}, req.LocalizedString("index.html"), "layouts/default.html")
}

// updateModuleVersionsCount updates the `moduleVersionCount`.
func updateModuleVersionsCount() error {
	cache, err := qiniuKodoCacher.Cache(base.Context, "stats/summary")
	if err != nil {
		return err
	}
	defer cache.Close()

	b, err := io.ReadAll(cache)
	if err != nil {
		return err
	}

	moduleVersionCount = gjson.GetBytes(b, "module_version_count").Int()

	return nil
}

// thousandsCommaSeperated returns a thousands comma seperated string for the n.
func thousandsCommaSeperated(n int64) string {
	in := strconv.FormatInt(n, 10)
	numOfDigits := len(in)
	if n < 0 {
		numOfDigits--
	}

	numOfCommas := (numOfDigits - 1) / 3

	out := make([]byte, len(in)+numOfCommas)
	if n < 0 {
		in, out[0] = in[1:], '-'
	}

	for i, j, k := len(in)-1, len(out)-1, 0; ; i, j = i-1, j-1 {
		out[j] = in[i]
		if i == 0 {
			return string(out)
		}
		if k++; k == 3 {
			j, k = j-1, 0
			out[j] = ','
		}
	}
}
