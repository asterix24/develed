package main

import (
	"bytes"
	"flag"
	"image"
	"image/color"
	"image/draw"
	"image/png"
	"net"
	"time"

	bitmapfont "github.com/develed/develed/bitmapfont"

	log "github.com/Sirupsen/logrus"
	"github.com/develed/develed/config"
	srv "github.com/develed/develed/services"
	"golang.org/x/net/context"
	"google.golang.org/grpc"
	"google.golang.org/grpc/reflection"
)

var (
	debug = flag.Bool("debug", false, "enter debug mode")
	cfg   = flag.String("config", "/etc/develed.toml", "configuration file")
)

var conf *config.Global

type server struct {
	sink srv.ImageSinkClient
}

type RenderCtx struct {
	img        image.Image
	charWidth  int
	scrollTime time.Duration
	efxType    string
}

var cRenderTextChannel = make(chan RenderCtx, 1)
var cSyncChannel = make(chan bool)
var cFrameWidth int = 39
var cFrameHigh int = 9
var cScollText string = "scroll"
var cFixText string = "fix"
var cCenterText string = "center"
var cBlinkText string = "blink"

func (s *server) Write(ctx context.Context, req *srv.TextRequest) (*srv.TextResponse, error) {
	var err error
	err = bitmapfont.Init(conf.Textd.FontPath, req.Font, conf.BitmapFonts)
	if err != nil {
		return &srv.TextResponse{
			Code:   -1,
			Status: err.Error(),
		}, nil
	}

	log.Debugf("Color: %v Bg: %v", req.FontColor, req.FontBg)
	txt_color := color.RGBA{255, 0, 0, 255}
	txt_bg := color.RGBA{0, 0, 0, 255}
	text_img, charWidth, err := bitmapfont.Render(req.Text, txt_color, txt_bg, 1, 0)
	if err != nil {
		return &srv.TextResponse{
			Code:   -1,
			Status: err.Error(),
		}, nil
	}

	cSyncChannel <- true
	cRenderTextChannel <- RenderCtx{text_img, charWidth, conf.Textd.TextScrollTime * time.Millisecond, "scroll"}

	return &srv.TextResponse{
		Code:   0,
		Status: "Ok",
	}, nil
}

func blitFrame(dr_srv *server, img image.Image, draw_rect image.Rectangle) error {
	frame := image.NewRGBA(image.Rect(0, 0, cFrameWidth, cFrameHigh))
	if img != nil {
		draw.Draw(frame, draw_rect, img, image.ZP, draw.Src)
	}
	buf := &bytes.Buffer{}
	png.Encode(buf, frame)
	_, err := dr_srv.sink.Draw(context.Background(), &srv.DrawRequest{
		Priority: int64(conf.Textd.Priority),
		Timeslot: 1,
		Data:     buf.Bytes(),
	})
	if err != nil {
		return err
	}
	return nil
}

func textRenderEfx(dr_srv *server, img image.Image, ctx RenderCtx) error {
	var err error
	switch ctx.efxType {
	case cScollText:
		for frame_idx := 0; ; frame_idx++ {
			// Scrolling time..
			time.Sleep(ctx.scrollTime)
			err = blitFrame(dr_srv, img, image.Rect(cFrameWidth-frame_idx, 0, cFrameWidth, cFrameHigh))
			if err != nil {
				return err
			}
			if frame_idx >= (img.Bounds().Dx() + cFrameWidth) {
				log.Debug("End frame wrap..")
				return nil
			}
		}
	case cFixText:
		err = blitFrame(dr_srv, img, image.Rect(0, 0, cFrameWidth, cFrameHigh))
		if err != nil {
			return err
		}
	case cCenterText:
		off := cFrameWidth - img.Bounds().Dx()
		if off > 0 {
			off = off / 2
		} else {
			off = 0
		}
		err = blitFrame(dr_srv, img, image.Rect(off, 0, cFrameWidth-off, cFrameHigh))
		if err != nil {
			return err
		}
	}
	return nil
}

func renderLoop(dr_srv *server) {
	ctx := RenderCtx{nil, cFrameWidth, 0, "fix"}
	text_img := image.NewRGBA(image.Rect(0, 0, cFrameWidth, cFrameHigh))
	//draw.Draw(text_img, text_img.Bounds(), &image.Uniform{color.RGBA{0, 255, 0, 255}}, image.ZP, draw.Src)

	for {
		select {
		case ctx = <-cRenderTextChannel:
			log.Debug("Text Render channel")
		default:
			// Message from a channel lets render it
			if ctx.img != nil {
				text_img = image.NewRGBA(ctx.img.Bounds())
				draw.Draw(text_img, ctx.img.Bounds(), ctx.img, image.ZP, draw.Src)
			}
			err := textRenderEfx(dr_srv, text_img, ctx)
			if err != nil {
				log.Error(err.Error())
			}
		}
	}
}

func main() {
	var err error

	flag.Parse()

	conf, err = config.Load(*cfg)
	if err != nil {
		log.Fatalln(err)
	}

	if *debug {
		log.SetLevel(log.DebugLevel)
	}

	sock, err := net.Listen("tcp", conf.Textd.GRPCServerAddress)
	if err != nil {
		log.Fatalln(err)
	}

	conn, err := grpc.Dial(conf.DSPD.GRPCServerAddress, grpc.WithInsecure())
	if err != nil {
		log.Fatalln(err)
	}
	defer conn.Close()

	s := grpc.NewServer()
	drawing_srv := &server{sink: srv.NewImageSinkClient(conn)}
	srv.RegisterTextdServer(s, drawing_srv)
	reflection.Register(s)

	go renderLoop(drawing_srv)

	if err := s.Serve(sock); err != nil {
		log.Fatalln(err)
	}
}
