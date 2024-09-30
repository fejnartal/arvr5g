package main

import (
	"encoding/gob"
	"fmt"
	webview "github.com/webview/webview_go"
	"os"
	"os/signal"
	"strconv"
	"syscall"
	"time"
)

type Progreso struct {
	NumSesiones  int
	NumRefrescos int
	TiempoAcumul time.Duration
}

var username = os.Getenv("username")
var password = os.Getenv("password")
var js = `
document.addEventListener('DOMContentLoaded', function() {
    var login = document.getElementById("login");
    var username = document.getElementById("username");
    var password = document.getElementById("password");

    var message = document.getElementById("message");
    var editing = document.getElementById("editing");

    if(login != null && username != null && password != null) {
        username.value = "` + username + `"
        password.value = "` + password + `"
        login.submit()
    } else if(message != null && editing != null) {
        setInterval(function() {
            window.comprobarMensajesNuevos().then(function(mensaje) {
                if(mensaje != "") {
                  message.value = mensaje;
		  reload();
                  //editing.submit()
                } else {
                  message.value = "No hay mensajes"
                }
            })
        }, 5 * 1000);

    } else {
        return
    }

}, false);
`
var horaInicio time.Time = time.Now()
var horaAnterior time.Time = horaInicio
var progreso Progreso
var mensajesPendientes []string

func mensajesPredefinidos(inicioFinal string, prog Progreso, minRefresco time.Duration) []string {
	return []string{
		fmt.Sprintf(`Este es el %s de la sesión automatizada nº %s`, inicioFinal, strconv.Itoa(prog.NumSesiones)),
		//		fmt.Sprintf(`La sesión 5 presentaba un bug en el cálculo de horas infladas que ya ha sido subsanado.`),
		fmt.Sprintf(`Hasta ahora he inflado artificalmente mi "Dedicación" en la plataforma %s`, prog.TiempoAcumul),
		fmt.Sprintf(`Para ello, cada %s se envia un mensaje automático al chat para evitar que Moodle cierre la sesión.`, minRefresco),
		`Aunque bastaría con refrescar la página (sin dejar huella) para evitar que Moodle cierre la sesión`,
		`he preferido dejar constancia de ello puesto que esta forma de inflar el tiempo de "Dedicación"`,
		`ha sido autorizada y promovida por la empresa que imparte el curso argumentando que el contenido de la plataforma no da para cubrir las 120 horas del programa formativo:`,
		`https://sede.sepe.gob.es/es/portaltrabaja/resources/pdf/especialidades/IFCD102.pdf`,
		`También aseguran que están tratando de convencer (sin éxito) a los inspectores de estos hechos`,
		`para que no exijan que la "Dedicación" registrada en la plataforma tenga que llegar a las 120 horas.`,
		`Desde mi punto de vista el principal problema es la ausencia de contenido de calidad en la plataforma.`,
		`Creo que es posible (exigible incluso) ofrecer 120 horas de contenido de calidad en un curso cuyo programa formativo reza:`,

		`OBJETIVO GENERAL: Programar en C# en Unity para la realización de software de realidad virtual y aumentada en entornos de cobertura 5G.`,
		`EXPERIENCIA PROFESIONAL: No se requiere`,

		`Sin embargo, los contenidos presentes en la plataforman son absolutamente insuficientes y de ínfima calidad`,
		`tanto para personas sin experiencia profesional previa en programación como para profesionales del sector.`,
		`Pueden encontrar el código fuente que automatiza estos mensajes en https://github.com/fejnartal/arvr5g`,
	}
}

func main() {
	progreso = cargarProgreso()
	progreso.NumSesiones += 1
	limiteGHActions := time.Duration(5) * time.Hour
	minutosRefresco := time.Duration(5) * time.Minute

	mensajesPendientes = mensajesPredefinidos("INICIO", progreso, minutosRefresco)

	w := webview.New(true)
	defer w.Destroy()

	ticker := time.NewTicker(minutosRefresco)
	defer ticker.Stop()
	go func() {
		for {
			<-ticker.C
			actualizarProgreso()
		}
	}()

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, os.Interrupt, syscall.SIGINT, syscall.SIGTERM)

	timer := time.NewTimer(limiteGHActions)
	defer timer.Stop()
	go func() {
		<-timer.C
		// Cuando nos estemos acercando al límite de ejecución de GitHub Actions iniciamos un cierre controlado del programa
		quit <- syscall.SIGINT
	}()

	go func() {
		<-quit
		fmt.Fprintf(os.Stdout, "Recibida señal de cierre controlado del programa")
		ticker.Stop()
		mensajesPendientes = append(mensajesPendientes, mensajesPredefinidos("FINAL", progreso, minutosRefresco)...)

		fmt.Fprintf(os.Stdout, "Intentamos esperar hasta que no haya mensajes pendientes antes de terminar el programa")
		for {
			if len(mensajesPendientes) == 0 {
				//fmt.Fprintf(os.Stdout, "No quedan mensajes pendientes. Terminamos el programa")
				w.Unbind("comprobarMensajesNuevos")
				w.Terminate()
				os.Exit(0)
			}
		}
	}()

	w.Bind("comprobarMensajesNuevos", func() string {
		fmt.Fprintf(os.Stdout, "Comprobando mensajes nuevos: %s", time.Now())

		var nuevoMensaje string
		if len(mensajesPendientes) == 0 {
			nuevoMensaje = ""
		} else if len(mensajesPendientes) == 1 {
			nuevoMensaje = mensajesPendientes[0]
			mensajesPendientes = []string{}
		} else {
			nuevoMensaje = mensajesPendientes[0]
			mensajesPendientes = mensajesPendientes[1:]
		}
		return nuevoMensaje
	})
	defer w.Unbind("comprobarMensajesNuevos")

	w.Init(js)
	w.Navigate("https://aulavirtual.integraconocimiento.es/mod/chat/gui_basic/index.php?id=" + os.Getenv("idcurso"))
	w.Run()
}

func actualizarProgreso() {
	horaActual := time.Now()
	tiempoTranscurrido := horaActual.Sub(horaAnterior)
	horaAnterior = horaActual
	progreso.NumRefrescos += 1
	progreso.TiempoAcumul += tiempoTranscurrido

	if len(mensajesPendientes) != 0 {
		fmt.Fprintf(os.Stderr, "Algo ha fallado y no se están enviando las respuestas a la web")
		os.Exit(2)
	} else {
		mensajesPendientes = append(mensajesPendientes, fmt.Sprintf("Sesión: %s | AutorecargasTotales: %s | DedicaciónInflada: %s", strconv.Itoa(progreso.NumSesiones), strconv.Itoa(progreso.NumRefrescos), progreso.TiempoAcumul))
	}
	salvarProgreso(progreso)
}

func cargarProgreso() Progreso {
	f, err := os.OpenFile("integra5garvr.bin", os.O_RDONLY, 0644)
	if err != nil {
		fmt.Fprintf(os.Stderr, "No se pudo abrir el fichero integra5garvr.bin: %v", err)
	}
	defer f.Close()
	dec := gob.NewDecoder(f)
	var progreso Progreso
	err = dec.Decode(&progreso)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Lectura fallida: %v", err)
		os.Exit(2)
	}
	return progreso
}

func salvarProgreso(progreso Progreso) {
	f, err := os.OpenFile("integra5garvr.bin", os.O_TRUNC|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		fmt.Fprintf(os.Stderr, "No se pudo abrir el fichero integra5garvr.bin: %v", err)
		os.Exit(2)
	}
	defer f.Close()
	enc := gob.NewEncoder(f)
	err = enc.Encode(progreso)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Escritura fallida: %v", err)
		os.Exit(2)
	}
}
