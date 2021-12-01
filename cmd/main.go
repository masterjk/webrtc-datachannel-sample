package main

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"os"
	"time"

	"github.com/gorilla/mux"
	"github.com/pion/webrtc/v3"

	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
)

func Encode(obj interface{}) string {
	b, err := json.Marshal(obj)
	if err != nil {
		panic(err)
	}

	// if compress {
	// 	b = zip(b)
	// }

	return base64.StdEncoding.EncodeToString(b)
}

func createPeerConnection(sdpOffer string) *webrtc.PeerConnection {
	config := webrtc.Configuration{
		ICEServers: []webrtc.ICEServer{
			{
				URLs: []string{"stun:stun.l.google.com:19302"},
			},
		},
	}

	api := webrtc.NewAPI()

	peerConnection, err := api.NewPeerConnection(config)
	if err != nil {
		panic(err)
	}

	peerConnection.OnConnectionStateChange(func(state webrtc.PeerConnectionState) {
		log.Debug().Msgf("OnConnectionStateChange %+v", state)
	})

	peerConnection.OnDataChannel(func(dataChannel *webrtc.DataChannel) {

		dataChannel.OnOpen(func() {
			log.Debug().Msgf("OnDataChannel %+v", dataChannel)

			for {
				sendText := fmt.Sprintf("Time now is %s", time.Now().Format(time.RFC3339))
				err := dataChannel.SendText(sendText)
				if err != nil {
					log.Error().Err(err).Msg("Error sending via data channel")
				}
				time.Sleep(1 * time.Second)
			}
		})

	})

	peerConnection.OnICECandidate(func(candidate *webrtc.ICECandidate) {
		log.Debug().Msgf("OnICECandidate %+v", candidate)
	})

	peerConnection.OnICEConnectionStateChange(func(connectionState webrtc.ICEConnectionState) {
		log.Debug().Msgf("OnICEConnectionStateChange %+v", connectionState)
	})

	peerConnection.OnICEGatheringStateChange(func(state webrtc.ICEGathererState) {
		log.Debug().Msgf("OnICEGatheringStateChange %+v", state)
	})

	peerConnection.OnNegotiationNeeded(func() {
		log.Debug().Msgf("OnNegotiationNeeded")
	})

	peerConnection.OnSignalingStateChange(func(signalingState webrtc.SignalingState) {
		log.Debug().Msgf("OnSignalingStateChange %+v", signalingState)
	})

	peerConnection.OnTrack(func(track *webrtc.TrackRemote, receiver *webrtc.RTPReceiver) {
		log.Debug().Msgf("OnTrack track: %+v; receiver: %+v", track, receiver)
	})

	offer := webrtc.SessionDescription{}
	offer.Type = webrtc.SDPTypeOffer
	offer.SDP = sdpOffer

	log.Info().Msgf("***** SDP Offer received *****\n%s***** SDP Offer END *****\n\n", sdpOffer)

	// Set the remote SessionDescription
	err = peerConnection.SetRemoteDescription(offer)
	if err != nil {
		panic(err)
	}

	// Create an answer
	answer, err := peerConnection.CreateAnswer(nil)
	if err != nil {
		panic(err)
	}

	// Create channel that is blocked until ICE Gathering is complete
	gatherComplete := webrtc.GatheringCompletePromise(peerConnection)

	// Sets the LocalDescription, and starts our UDP listeners
	err = peerConnection.SetLocalDescription(answer)
	if err != nil {
		panic(err)
	}

	<-gatherComplete

	// answer
	return peerConnection
}

func handler(w http.ResponseWriter, r *http.Request) {

	body, _ := ioutil.ReadAll(r.Body)
	peerConnection := createPeerConnection(string(body))

	log.Debug().Msgf("***** SDP Answer Created *****\n%s\n***** SDP Answer END *****\n\n", peerConnection.LocalDescription().SDP)

	sdpAnswer := Encode(peerConnection.LocalDescription())

	// peerConnection.LocalDescription().
	w.WriteHeader(http.StatusOK)
	w.Write([]byte(sdpAnswer))
}

func main() {

	log.Logger = log.Output(zerolog.ConsoleWriter{Out: os.Stdout, TimeFormat: time.RFC3339})
	zerolog.TimeFieldFormat = zerolog.TimeFormatUnix

	r := mux.NewRouter()

	r.HandleFunc("/sdp", handler).Methods("POST")
	r.PathPrefix("/").Handler(http.FileServer(http.Dir("./web/")))

	srv := &http.Server{
		Handler:      r,
		Addr:         "0.0.0.0:8080",
		WriteTimeout: 15 * time.Second,
		ReadTimeout:  15 * time.Second,
	}

	log.Info().Msg("Started server...")

	srv.ListenAndServe()
}
