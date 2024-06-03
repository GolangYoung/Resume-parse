package routers

import (
	"tempgo/controllers"

	"github.com/astaxie/beego"
)

func init() {
	beego.Router("/", &controllers.PDFController{})
}
