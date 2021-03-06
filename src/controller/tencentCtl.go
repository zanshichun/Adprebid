package controller

import (
	"bytes"
	"fmt"
	"github.com/golang/protobuf/proto"
	"github.com/wxnacy/wgo/arrays"
	"io/ioutil"
	"math/rand"
	"net/http"
	"net/http/httputil"
	"net/url"
	"strconv"
	"strings"
	"sync"
	"tencentgo/src/helpers/config"
	tencent "tencentgo/src/model/tencent"
)

type upStreamStruct struct {
	ipAddr string
	deals  []string
}

type TencentHandler struct {
	addrs []string
}

type bodyContent struct {
	body string
	cnt  int
}

var (
	bodyMap = &sync.Map{}
	u1      upStreamStruct
	u2      upStreamStruct
	u3      upStreamStruct
	u4      upStreamStruct

	deals1 []string
	deals2 []string
	deals3 []string
	deals4 []string
	deals5 []string
	deals6 []string //将所有的dealId

	tencentConfig config.Tencent
)

type transport struct {
	http.RoundTripper
}

var _ http.RoundTripper = &transport{}

func contains(s []string, e string, isExact bool) bool {
	for _, a := range s {
		a = strings.TrimSpace(a)
		if isExact {
			if a == e {
				return true
			}
		} else {
			if strings.Contains(a, e) || strings.Contains(e, a) {
				return true
			}
		}
	}
	return false
}

func (t *transport) RoundTrip(req *http.Request) (resp *http.Response, err error) {
	// copy request
	b, err := ioutil.ReadAll(req.Body)

	//process request change
	b = bytes.Replace(b, []byte("server"), []byte("schmerver"), -1)
	newRequest := &tencent.Request{}
	err = proto.Unmarshal(b, newRequest)

	//modify b if necessary

	//turn to pb and set back
	data, err := proto.Marshal(newRequest) //TODO: if no changed, just send original pb to http
	body := ioutil.NopCloser(bytes.NewReader(data))
	req.Body = body
	req.ContentLength = int64(len(data))
	req.Header.Set("Content-Length", strconv.Itoa(len(data)))

	//set back
	//req.Body = ioutil.NopCloser(bytes.NewBuffer(b))

	// reverse proxy
	resp, err = t.RoundTripper.RoundTrip(req)

	// error to nil return
	if err != nil {
		return nil, err
	}
	err = req.Body.Close()
	if err != nil {
		return nil, err
	}

	//TODO: should be error return here, to find out a new solution

	b, err = ioutil.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}
	err = resp.Body.Close()
	if err != nil {
		return nil, err
	}
	b = bytes.Replace(b, []byte("server"), []byte("schmerver"), -1)
	//body = ioutil.NopCloser(bytes.NewReader(b))
	newResponse := &tencent.Response{}

	err = proto.Unmarshal(b, newResponse)
	dealid := newRequest.Impression[0].GetDealid()
	if len(newResponse.GetSeatbid()) > 0 && len(newResponse.GetSeatbid()[0].GetBid()) > 0 {
		adid := newResponse.Seatbid[0].Bid[0].GetAdid()
		//mutex.Lock()
		bodyMap.Store(dealid, bodyContent{adid, 0})
		//mutex.Unlock()
		*newResponse.GetSeatbid()[0].GetBid()[0].Ext = "ssp" + adid
	} else {
		//mutex.Lock()
		bodyMap.Store(dealid, bodyContent{"0", 1})
		//mutex.Unlock()
	}

	fmt.Println("REQREQREQREQ\n" + newRequest.String())
	fmt.Println("RESPRESPRESPRESP\n" + newResponse.String())

	// pb object to response body and return to hhtp
	data, err = proto.Marshal(newResponse) //TODO: if no changed, just send original pb to http
	body = ioutil.NopCloser(bytes.NewReader(data))
	resp.Body = body
	resp.ContentLength = int64(len(data))
	resp.Header.Set("Content-Length", strconv.Itoa(len(data)))
	return resp, nil
}

func (this *TencentHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {

	b, err := ioutil.ReadAll(r.Body)

	//process request change
	b = bytes.Replace(b, []byte("server"), []byte("schmerver"), -1)
	newRequest := &tencent.Request{}
	err = proto.Unmarshal(b, newRequest)

	addr := tencentConfig.DefaultUpstreamAddr
	//if newRequest.Device.DeviceId != nil {

	newRequestDealId := newRequest.GetImpression()[0].GetDealid()

	if contains(deals1, newRequestDealId, false) {
		addr = u1.ipAddr
	} else if contains(deals2, newRequestDealId, false) {
		addr = u2.ipAddr
	} else if contains(deals3, newRequestDealId, false) {
		addr = u3.ipAddr
	} else if contains(deals4, newRequestDealId, false) {
		addr = u4.ipAddr
	} /*else if contains(deals5, newRequestDealId, false) {
		addr = rconfig.UpstreamAddr5
	}*/
	//}

	remote, err := url.Parse("http://" + addr)
	if err != nil {
		panic(err)
	}
	// if  in bodyMap,return body directly
	dealid := newRequest.Impression[0].GetDealid()
	//mutex.Lock()
	bodycontent, ok := bodyMap.Load(dealid)
	//mutex.Unlock()

	if ok && arrays.Contains(deals6, dealid) != -1 && bodycontent != nil {
		//rand.Seed(time.Now().UnixNano())
		if rand.Intn(tencentConfig.TimesBackToSource) > 1 {
			fmt.Println(newRequestDealId + " ==>" + addr)
			id := newRequest.GetId()
			bidid := newRequest.Impression[0].GetId()
			adid := bodycontent.(bodyContent).body
			price := float32(9000)
			extid := "ssp" + adid
			//mutex.Lock()
			//bodyMap[dealid] = bodyContent{adid, bodycontent.cnt + 1}
			//mutex.Unlock()
			err = proto.Unmarshal(b, newRequest)
			newResponse := &tencent.Response{}
			if adid != "0" {
				newResponse = &tencent.Response{
					Id: &id,
					Seatbid: []*tencent.Response_SeatBid{
						{
							Bid: []*tencent.Response_Bid{
								{Id: &bidid,
									Impid: &bidid,
									Price: &price,
									Adid:  &adid,
									Ext:   &extid},
							},
						},
					},
				}
			} else {
				newResponse = &tencent.Response{
					Id: &id,
				}
			}
			data, err := proto.Marshal(newResponse) //TODO: if no changed, just send original pb to http
			if err != nil {
				w.WriteHeader(204)
			}
			//bodyMap[*(newRequest.Impression[0].Dealid)] = bodyContent{bodycontent.body, bodycontent.cnt + 1}
			w.Write(data)

			fmt.Println("REQREQREQREQ\n" + newRequest.String())
			fmt.Println("RESPRESPRESPRESP\n" + newResponse.String())
			return
		}
	}

	//if not in bodyMap, reverseProxy and transpot RoundTrip,
	body := ioutil.NopCloser(bytes.NewReader(b))
	r.Body = body
	proxy := httputil.NewSingleHostReverseProxy(remote)
	proxy.Transport = &transport{http.DefaultTransport}
	proxy.ServeHTTP(w, r)

}

func TencentCtlInit() {

	//配置文件初始化
	tencentConfig = config.MediaConf.Tencent

	if len(tencentConfig.UpstreamAddrs) != 0 {
		for _, v := range tencentConfig.UpstreamAddrs {

			upStream := strings.Split(v, "|")
			id := upStream[0]
			usSplit := strings.Split(upStream[1], ",")
			deals := usSplit[1:]
			uss := &upStreamStruct{
				ipAddr: usSplit[0],
				deals:  deals,
			}
			switch id {
			case "1":
				u1 = *uss
				deals1 = u1.deals
			case "2":
				u2 = *uss
				deals2 = u2.deals
			case "3":
				u3 = *uss
				deals3 = u3.deals
			case "4":
				deals4 = u4.deals
				u4 = *uss
			}

			deals6 = append(deals6, deals...)

		}
	}

}
