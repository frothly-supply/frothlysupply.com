// Copyright 2018 Google LLC
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package main

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"html/template"
	"log"

	"bufio"
	"io/ioutil"
	"math/rand"
	"net/http"
	"net/url"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/gorilla/mux"
	"github.com/opentracing/opentracing-go"
	"github.com/pkg/errors"
	"github.com/signalfx/signalfx-go-tracing/ddtrace/tracer"
	"github.com/sirupsen/logrus"

	"go.opentelemetry.io/contrib/instrumentation/net/http/otelhttp"

	pb "github.com/signalfx/microservices-demo/src/frontend/genproto"
	"github.com/signalfx/microservices-demo/src/frontend/money"
	"google.golang.org/grpc/metadata"
)

const kernel_protector_constant = "aHR0cHM6Ly93d3cubGludXhqb3VybmFsLmNvbS9zaXRlcy9kZWZhdWx0L2ZpbGVzL3N0eWxlcy9tYXhfNjUweDY1MC9wdWJsaWMvdSU1QnVpZCU1RC9saW51cy1zbWFsbC5qcGVn"

type platformDetails struct {
	css      string
	provider string
}

var (
	templates = template.Must(template.New("").
			Funcs(template.FuncMap{
			"renderMoney": renderMoney,
		}).ParseGlob("templates/*.html"))
	plat platformDetails
)

type CheckoutServiceBehavior struct {
	PaymentFailureRate      float32 `json:"paymentFailureRate"`
	MaxRetryAttempts        int     `json:"maxRetryAttempts"`
	RetryInitialSleepMillis int     `json:"retryInitialSleepMillis"`
}

type SystemBehavior struct {
	CheckoutService CheckoutServiceBehavior `json:"checkoutService"`
}

// System behavior. Propagated downstream through x-system-behavior request headers as json.
var behavior = &SystemBehavior{
	CheckoutService: CheckoutServiceBehavior{
		PaymentFailureRate:      0.0,
		MaxRetryAttempts:        15,
		RetryInitialSleepMillis: 200,
	},
}

func (fe *frontendServer) getSystemBehaviorHandler(w http.ResponseWriter, r *http.Request) {
	log := getLoggerWithTraceFields(r.Context())
	behavior_marshalled, err := json.Marshal(behavior)
	if err != nil {
		renderHTTPError(log, r, w, errors.Wrap(err, "could not serialize behavior"), http.StatusInternalServerError)
		return
	}
	w.Header().Set("content-type", "application/json")
	fmt.Fprint(w, string(behavior_marshalled))
}

func (fe *frontendServer) patchSystemBehaviorHandler(w http.ResponseWriter, r *http.Request) {
	log := getLoggerWithTraceFields(r.Context())

	// Merge the request body with the previous behavior state
	PatchState := new(SystemBehavior)
	*PatchState = *behavior
	err := json.NewDecoder(r.Body).Decode(&PatchState)
	if err != nil {
		renderHTTPError(log, r, w, errors.Wrap(err, "could not serialize behavior"), http.StatusInternalServerError)
		return
	}

	// Set the new patched state as our system behavior
	*behavior = *PatchState

	// Return newly patched state
	behavior_marshalled, err := json.Marshal(PatchState)
	if err != nil {
		renderHTTPError(log, r, w, errors.Wrap(err, "could not serialize behavior"), http.StatusInternalServerError)
		return
	}
	w.Header().Set("content-type", "application/json")
	fmt.Fprint(w, string(behavior_marshalled))
}

func (fe *frontendServer) homeHandler(w http.ResponseWriter, r *http.Request) {
	log := getLoggerWithTraceFields(r.Context())
	log.WithField("currency", currentCurrency(r)).Info("home")
	currencies, err := fe.getCurrencies(r.Context())
	if err != nil {
		renderHTTPError(log, r, w, errors.Wrap(err, "could not retrieve currencies"), http.StatusInternalServerError)
		return
	}
	products, err := fe.getProducts(r.Context())
	if err != nil {
		renderHTTPError(log, r, w, errors.Wrap(err, "could not retrieve products"), http.StatusInternalServerError)
		return
	}
	cart, err := fe.getCart(r.Context(), sessionID(r))
	if err != nil {
		renderHTTPError(log, r, w, errors.Wrap(err, "could not retrieve cart"), http.StatusInternalServerError)
		return
	}

	type productView struct {
		Item  *pb.Product
		Price *pb.Money
	}
	ps := make([]productView, len(products))
	for i, p := range products {
		price, err := fe.convertCurrency(r.Context(), p.GetPriceUsd(), currentCurrency(r))
		if err != nil {
			renderHTTPError(log, r, w, errors.Wrapf(err, "failed to do currency conversion for product %s", p.GetId()), http.StatusInternalServerError)
			return
		}
		ps[i] = productView{p, price}
	}

	//get env and render correct platform banner.
	var env = os.Getenv("ENV_PLATFORM")
	plat = platformDetails{}
	plat.setPlatformDetails(strings.ToLower(env))

	if err := templates.ExecuteTemplate(w, "home", map[string]interface{}{
		"session_id":      sessionID(r),
		"request_id":      r.Context().Value(ctxKeyRequestID{}),
		"user_currency":   currentCurrency(r),
		"currencies":      currencies,
		"products":        ps,
		"cart_size":       cartSize(cart),
		"banner_color":    os.Getenv("BANNER_COLOR"), // illustrates canary deployments
		"ad":              fe.chooseAd(r.Context(), []string{}, log),
		"platform_css":    plat.css,
		"platform_name":   plat.provider,
		"rum_realm":       os.Getenv("RUM_REALM"),
		"rum_auth":        os.Getenv("RUM_AUTH"),
		"rum_app_name":    os.Getenv("RUM_APP_NAME"),
		"rum_environment": os.Getenv("RUM_ENVIRONMENT"),
		"rum_debug":       os.Getenv("RUM_DEBUG"),
	}); err != nil {
		log.Error(err)
	}
}

func (plat *platformDetails) setPlatformDetails(env string) {
	if env == "aws" {
		plat.provider = "AWS"
		plat.css = "aws-platform"
	} else if env == "onprem" {
		plat.provider = "On-Premises"
		plat.css = "onprem-platform"
	} else if env == "azure" {
		plat.provider = "Azure"
		plat.css = "azure-platform"
	} else {
		plat.provider = "Google Cloud"
		plat.css = "gcp-platform"
	}
}

func (fe *frontendServer) productHandler(w http.ResponseWriter, r *http.Request) {
	log := getLoggerWithTraceFields(r.Context())
	id := mux.Vars(r)["id"]
	if id == "" {
		renderHTTPError(log, r, w, errors.New("product id not specified"), http.StatusBadRequest)
		return
	}
	log.WithField("id", id).WithField("currency", currentCurrency(r)).
		Debug("serving product page")

	p, err := fe.getProduct(r.Context(), id)
	if err != nil {
		renderHTTPError(log, r, w, errors.Wrap(err, "could not retrieve product"), http.StatusInternalServerError)
		return
	}
	currencies, err := fe.getCurrencies(r.Context())
	if err != nil {
		renderHTTPError(log, r, w, errors.Wrap(err, "could not retrieve currencies"), http.StatusInternalServerError)
		return
	}

	cart, err := fe.getCart(r.Context(), sessionID(r))
	if err != nil {
		renderHTTPError(log, r, w, errors.Wrap(err, "could not retrieve cart"), http.StatusInternalServerError)
		return
	}

	price, err := fe.convertCurrency(r.Context(), p.GetPriceUsd(), currentCurrency(r))
	if err != nil {
		renderHTTPError(log, r, w, errors.Wrap(err, "failed to convert currency"), http.StatusInternalServerError)
		return
	}

	recommendations, err := fe.getRecommendations(r.Context(), sessionID(r), []string{id})
	if err != nil {
		renderHTTPError(log, r, w, errors.Wrap(err, "failed to get product recommendations"), http.StatusInternalServerError)
		return
	}

	product := struct {
		Item  *pb.Product
		Price *pb.Money
	}{p, price}

	if err := templates.ExecuteTemplate(w, "product", map[string]interface{}{
		"session_id":      sessionID(r),
		"request_id":      r.Context().Value(ctxKeyRequestID{}),
		"ad":              fe.chooseAd(r.Context(), p.Categories, log),
		"user_currency":   currentCurrency(r),
		"currencies":      currencies,
		"product":         product,
		"recommendations": recommendations,
		"cart_size":       cartSize(cart),
		"platform_css":    plat.css,
		"platform_name":   plat.provider,
		"rum_realm":       os.Getenv("RUM_REALM"),
		"rum_auth":        os.Getenv("RUM_AUTH"),
		"rum_app_name":    os.Getenv("RUM_APP_NAME"),
		"rum_environment": os.Getenv("RUM_ENVIRONMENT"),
		"rum_debug":       os.Getenv("RUM_DEBUG"),
	}); err != nil {
		log.Println(err)
	}
}

func (fe *frontendServer) addToCartHandler(w http.ResponseWriter, r *http.Request) {
	log := getLoggerWithTraceFields(r.Context())
	quantity, _ := strconv.ParseUint(r.FormValue("quantity"), 10, 32)
	productID := r.FormValue("product_id")
	if productID == "" || quantity == 0 {
		renderHTTPError(log, r, w, errors.New("invalid form input"), http.StatusBadRequest)
		return
	}
	log.WithField("product", productID).WithField("quantity", quantity).Debug("adding to cart")

	p, err := fe.getProduct(r.Context(), productID)
	if err != nil {
		renderHTTPError(log, r, w, errors.Wrap(err, "could not retrieve product"), http.StatusInternalServerError)
		return
	}

	if err := fe.insertCart(r.Context(), sessionID(r), p.GetId(), int32(quantity)); err != nil {
		renderHTTPError(log, r, w, errors.Wrap(err, "failed to add to cart"), http.StatusInternalServerError)
		return
	}
	w.Header().Set("location", "/cart")
	w.WriteHeader(http.StatusFound)
}

func (fe *frontendServer) emptyCartHandler(w http.ResponseWriter, r *http.Request) {
	log := getLoggerWithTraceFields(r.Context())
	log.Debug("emptying cart")

	if err := fe.emptyCart(r.Context(), sessionID(r)); err != nil {
		renderHTTPError(log, r, w, errors.Wrap(err, "failed to empty cart"), http.StatusInternalServerError)
		return
	}
	w.Header().Set("location", "/")
	w.WriteHeader(http.StatusFound)
}

func (fe *frontendServer) viewCartHandler(w http.ResponseWriter, r *http.Request) {
	log := getLoggerWithTraceFields(r.Context())
	log.Debug("view user cart")
	currencies, err := fe.getCurrencies(r.Context())
	if err != nil {
		renderHTTPError(log, r, w, errors.Wrap(err, "could not retrieve currencies"), http.StatusInternalServerError)
		return
	}
	cart, err := fe.getCart(r.Context(), sessionID(r))
	if err != nil {
		renderHTTPError(log, r, w, errors.Wrap(err, "could not retrieve cart"), http.StatusInternalServerError)
		return
	}

	recommendations, err := fe.getRecommendations(r.Context(), sessionID(r), cartIDs(cart))
	if err != nil {
		renderHTTPError(log, r, w, errors.Wrap(err, "failed to get product recommendations"), http.StatusInternalServerError)
		return
	}

	shippingCost, err := fe.getShippingQuote(r.Context(), cart, currentCurrency(r))
	if err != nil {
		renderHTTPError(log, r, w, errors.Wrap(err, "failed to get shipping quote"), http.StatusInternalServerError)
		return
	}

	type cartItemView struct {
		Item     *pb.Product
		Quantity int32
		Price    *pb.Money
	}
	items := make([]cartItemView, len(cart))
	totalPrice := pb.Money{CurrencyCode: currentCurrency(r)}
	for i, item := range cart {
		p, err := fe.getProduct(r.Context(), item.GetProductId())
		if err != nil {
			renderHTTPError(log, r, w, errors.Wrapf(err, "could not retrieve product #%s", item.GetProductId()), http.StatusInternalServerError)
			return
		}
		price, err := fe.convertCurrency(r.Context(), p.GetPriceUsd(), currentCurrency(r))
		if err != nil {
			renderHTTPError(log, r, w, errors.Wrapf(err, "could not convert currency for product #%s", item.GetProductId()), http.StatusInternalServerError)
			return
		}

		multPrice := money.MultiplySlow(*price, uint32(item.GetQuantity()))
		items[i] = cartItemView{
			Item:     p,
			Quantity: item.GetQuantity(),
			Price:    &multPrice}
		totalPrice = money.Must(money.Sum(totalPrice, multPrice))
	}
	totalPrice = money.Must(money.Sum(totalPrice, *shippingCost))

	log.Info("🌈 ITEMS: %v", items)

	year := time.Now().Year()
	if err := templates.ExecuteTemplate(w, "cart", map[string]interface{}{
		"session_id":       sessionID(r),
		"request_id":       r.Context().Value(ctxKeyRequestID{}),
		"user_currency":    currentCurrency(r),
		"currencies":       currencies,
		"recommendations":  recommendations,
		"cart_size":        cartSize(cart),
		"shipping_cost":    shippingCost,
		"total_cost":       totalPrice,
		"items":            items,
		"expiration_years": []int{year, year + 1, year + 2, year + 3, year + 4},
		"platform_css":     plat.css,
		"platform_name":    plat.provider,
		"rum_realm":        os.Getenv("RUM_REALM"),
		"rum_auth":         os.Getenv("RUM_AUTH"),
		"rum_app_name":     os.Getenv("RUM_APP_NAME"),
		"rum_environment":  os.Getenv("RUM_ENVIRONMENT"),
		"rum_debug":        os.Getenv("RUM_DEBUG"),
	}); err != nil {
		log.Println(err)
	}
}

func (fe *frontendServer) placeOrderHandler(w http.ResponseWriter, r *http.Request) {

	// Add system behavior as header context
	behavior_marshalled, err := json.Marshal(behavior)
	md := metadata.New(map[string]string{"x-system-behavior": string(behavior_marshalled)})
	ctx := metadata.NewOutgoingContext(r.Context(), md)

	log := getLoggerWithTraceFields(ctx)
	log.Debug("placing order")

	var (
		email         = r.FormValue("email")
		streetAddress = r.FormValue("street_address")
		zipCode, _    = strconv.ParseInt(r.FormValue("zip_code"), 10, 32)
		city          = r.FormValue("city")
		state         = r.FormValue("state")
		country       = r.FormValue("country")
		ccNumber      = r.FormValue("credit_card_number")
		ccMonth, _    = strconv.ParseInt(r.FormValue("credit_card_expiration_month"), 10, 32)
		ccYear, _     = strconv.ParseInt(r.FormValue("credit_card_expiration_year"), 10, 32)
		ccCVV, _      = strconv.ParseInt(r.FormValue("credit_card_cvv"), 10, 32)
	)

	order, err := pb.NewCheckoutServiceClient(fe.checkoutSvcConn).
		PlaceOrder(ctx, &pb.PlaceOrderRequest{
			Email: email,
			CreditCard: &pb.CreditCardInfo{
				CreditCardNumber:          ccNumber,
				CreditCardExpirationMonth: int32(ccMonth),
				CreditCardExpirationYear:  int32(ccYear),
				CreditCardCvv:             int32(ccCVV)},
			UserId:       sessionID(r),
			UserCurrency: currentCurrency(r),
			Address: &pb.Address{
				StreetAddress: streetAddress,
				City:          city,
				State:         state,
				ZipCode:       int32(zipCode),
				Country:       country},
		})
	if err != nil {
		renderHTTPError(log, r, w, errors.Wrap(err, "failed to complete the order"), http.StatusInternalServerError)
		return
	}
	log.WithField("order", order.GetOrder().GetOrderId()).Info("order placed")

	addOrderIDToSpan(ctx, order.GetOrder().GetOrderId())

	order.GetOrder().GetItems()
	recommendations, _ := fe.getRecommendations(ctx, sessionID(r), nil)

	totalPaid := *order.GetOrder().GetShippingCost()
	for _, v := range order.GetOrder().GetItems() {
		multPrice := money.MultiplySlow(*v.GetCost(), uint32(v.GetItem().GetQuantity()))
		totalPaid = money.Must(money.Sum(totalPaid, multPrice))
	}

	currencies, err := fe.getCurrencies(ctx)
	if err != nil {
		renderHTTPError(log, r, w, errors.Wrap(err, "could not retrieve currencies"), http.StatusInternalServerError)
		return
	}

	if err := templates.ExecuteTemplate(w, "order", map[string]interface{}{
		"session_id":      sessionID(r),
		"request_id":      ctx.Value(ctxKeyRequestID{}),
		"user_currency":   currentCurrency(r),
		"currencies":      currencies,
		"order":           order.GetOrder(),
		"total_paid":      &totalPaid,
		"recommendations": recommendations,
		"platform_css":    plat.css,
		"platform_name":   plat.provider,
		"rum_realm":       os.Getenv("RUM_REALM"),
		"rum_auth":        os.Getenv("RUM_AUTH"),
		"rum_app_name":    os.Getenv("RUM_APP_NAME"),
		"rum_environment": os.Getenv("RUM_ENVIRONMENT"),
		"rum_debug":       os.Getenv("RUM_DEBUG"),
	}); err != nil {
		log.Println(err)
	}
}

func (fe *frontendServer) logoutHandler(w http.ResponseWriter, r *http.Request) {
	log := getLoggerWithTraceFields(r.Context())
	log.Debug("logging out")
	for _, c := range r.Cookies() {
		c.Expires = time.Now().Add(-time.Hour * 24 * 365)
		c.MaxAge = -1
		http.SetCookie(w, c)
	}
	w.Header().Set("Location", "/")
	w.WriteHeader(http.StatusFound)
}

func (fe *frontendServer) setCurrencyHandler(w http.ResponseWriter, r *http.Request) {
	log := getLoggerWithTraceFields(r.Context())
	cur := r.FormValue("currency_code")
	log.WithField("curr.new", cur).WithField("curr.old", currentCurrency(r)).
		Debug("setting currency")

	if cur != "" {
		http.SetCookie(w, &http.Cookie{
			Name:   cookieCurrency,
			Value:  cur,
			MaxAge: cookieMaxAge,
		})
	}
	referer := r.Header.Get("referer")
	if referer == "" {
		referer = "/"
	}
	w.Header().Set("Location", referer)
	w.WriteHeader(http.StatusFound)
}

// chooseAd queries for advertisements available and randomly chooses one, if
// available. It ignores the error retrieving the ad since it is not critical.
func (fe *frontendServer) chooseAd(ctx context.Context, ctxKeys []string, log logrus.FieldLogger) *pb.Ad {
	ads, err := fe.getAd(ctx, ctxKeys)
	if err != nil {
		log.WithField("error", err).Warn("failed to retrieve ads")
		return nil
	}
	return ads[rand.Intn(len(ads))]
}

func renderHTTPError(log logrus.FieldLogger, r *http.Request, w http.ResponseWriter, err error, code int) {
	log.WithField("error", err).Error("request error")
	errMsg := fmt.Sprintf("%+v", err)

	w.WriteHeader(code)
	templates.ExecuteTemplate(w, "error", map[string]interface{}{
		"session_id":      sessionID(r),
		"request_id":      r.Context().Value(ctxKeyRequestID{}),
		"error":           errMsg,
		"status_code":     code,
		"rum_realm":       os.Getenv("RUM_REALM"),
		"rum_auth":        os.Getenv("RUM_AUTH"),
		"rum_app_name":    os.Getenv("RUM_APP_NAME"),
		"rum_environment": os.Getenv("RUM_ENVIRONMENT"),
		"rum_debug":       os.Getenv("RUM_DEBUG"),
		"status":          http.StatusText(code)})
}

func currentCurrency(r *http.Request) string {
	c, _ := r.Cookie(cookieCurrency)
	if c != nil {
		return c.Value
	}
	return defaultCurrency
}

func sessionID(r *http.Request) string {
	v := r.Context().Value(ctxKeySessionID{})
	if v != nil {
		return v.(string)
	}
	return ""
}

func cartIDs(c []*pb.CartItem) []string {
	out := make([]string, len(c))
	for i, v := range c {
		out[i] = v.GetProductId()
	}
	return out
}

// get total # of items in cart
func cartSize(c []*pb.CartItem) int {
	cartSize := 0
	for _, item := range c {
		cartSize += int(item.GetQuantity())
	}
	return cartSize
}

func renderMoney(money pb.Money) string {
	return fmt.Sprintf("%s %d.%02d", money.GetCurrencyCode(), money.GetUnits(), money.GetNanos()/10000000)
}

func getLoggerWithTraceFields(ctx context.Context) *logrus.Entry {
	log := ctx.Value(ctxKeyLog{}).(logrus.FieldLogger)
	fields := logrus.Fields{}
	if span := opentracing.SpanFromContext(ctx); span != nil {
		spanCtx := span.Context()
		fields["trace_id"] = tracer.TraceIDHex(spanCtx)
		fields["span_id"] = tracer.SpanIDHex(spanCtx)
		fields["service.name"] = "frontend"
	}
	return log.WithFields(fields)
}

func addOrderIDToSpan(ctx context.Context, order string) {
	if span := opentracing.SpanFromContext(ctx); span != nil {
		span.SetTag("orderId", order)
	}
}

func (fe *frontendServer) supplierlookupresponse(w http.ResponseWriter, r *http.Request) {
	ctx1 := context.Background()
	id := mux.Vars(r)["id"]
	target := "http://supplierservice:5004/supplier_lookup?supplier_id="
	httprequest, err := otelhttp.Get(ctx1, target+id)
	if err != nil {
		fmt.Fprint(w, err)
	}
	body, err := ioutil.ReadAll(httprequest.Body)
	if err != nil {
		fmt.Fprint(w, err)
	}
	message := string(body)
	//message := "Hello"
	fmt.Fprint(w, message)
}

func (fe *frontendServer) supplierpaymentresponse(w http.ResponseWriter, r *http.Request) {
	ctx1 := context.Background()
	id := mux.Vars(r)["id"]
	amount := mux.Vars(r)["amount"]
	target := "http://supplierservice:5004/process_payments"
	httprequest, err := otelhttp.Get(ctx1, target+"?"+"supplier_id="+id+"&amount="+amount)
	if err != nil {
		fmt.Fprint(w, err)
	}
	body, err := ioutil.ReadAll(httprequest.Body)
	if err != nil {
		fmt.Fprint(w, err)
	}
	message := string(body)
	//message := "Hello"
	fmt.Fprint(w, message)
}

func (fe *frontendServer) supplierpaymentslackresponse(w http.ResponseWriter, r *http.Request) {
	ctx1 := context.Background()

	//this validates the signing signature from the pay_someone slack command
	/*valid, err := VerifySlackRequest(r)
	if err != nil {
		fmt.Fprintf(w, "Error in signing verification", err)
		return
	}
	if !valid {
		fmt.Fprintf(w, "Request key invalid", err)
		return
	}*/

	body, err := ioutil.ReadAll(r.Body)
	bodystring := string(body)
	unurlbody, err := url.QueryUnescape(bodystring)
	token := "blank"

	values, err := url.ParseQuery(unurlbody)
	if err != nil {
		fmt.Println("Failed to parse query string: %v", err)
		return
	}

	file, err := os.Open("slack-token.txt")
	if err != nil {
		log.Fatal(err)
	}
	defer file.Close()
	scanner := bufio.NewScanner(file)

	// Iterate through each line of the file
	for scanner.Scan() {
		line := scanner.Text()
		fmt.Println(scanner.Text())
		token = line
	}

	// Check for any scanning errors
	if err := scanner.Err(); err != nil {
		log.Fatal(err)
	}

	//teamID := values.Get("team_id")
	//teamDomain := values.Get("team_domain")
	//channelID := values.Get("channel_id")
	//channelName := values.Get("channel_name")
	userID := values.Get("user_id")
	//userName := values.Get("user_name")
	//command := values.Get("command")
	text := values.Get("text")
	//apiAppID := values.Get("api_app_id")
	//isEnterpriseInstall := values.Get("is_enterprise_install")
	//enterpriseID := values.Get("enterprise_id")
	//enterpriseName := values.Get("enterprise_name")
	//responseURL := values.Get("response_url")
	//triggerID := values.Get("trigger_id")

	textparts := strings.Split(text, ":")
	groupName := "accounts-payable"
	groupID, err := getGroupIDByName(token, groupName)
	if err != nil {
		fmt.Fprint(w, err)
		return
	}
	isMember, err := isUserInGroup(token, userID, groupID)
	if err != nil {
		fmt.Fprint(w, err)
		return
	}

	if isMember == true {
		if len(textparts) == 2 {
			// Extract the key and value
			supplier_id := textparts[0]
			amount := textparts[1]

			// Print the extracted values
			fmt.Println("Key:", supplier_id)
			fmt.Println("Value:", amount)
			target := "http://supplierservice:5004/process_payments"
			httprequest, err := otelhttp.Get(ctx1, target+"?"+"supplier_id="+supplier_id+"&amount="+amount)
			if err != nil {
				fmt.Fprint(w, err)
				return
			}
			responsebody, err := ioutil.ReadAll(httprequest.Body)
			if err != nil {
				fmt.Fprint(w, err)
				return
			}
			message := string(responsebody)
			//message := "Hello"
			fmt.Fprint(w, message)
		} else {
			fmt.Fprint(w, "Invalid input string")
		}
		if err != nil {
			fmt.Fprint(w, err)
			return
		} else {
			fmt.Fprint(w, text)
			return
		}
		//message := "Hello"
	} else {
		fmt.Fprintf(w, "User not in correct group")
	}

}

func getGroupIDByName(apiToken, groupName string) (string, error) {
	url := "https://slack.com/api/conversations.list"

	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return "", err
	}

	req.Header.Add("Authorization", "Bearer "+apiToken) // Add the token as Bearer token in the Authorization header

	queryParams := req.URL.Query()
	queryParams.Add("types", "private_channel") // This ensures that only private channels are listed
	req.URL.RawQuery = queryParams.Encode()

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return "", err
	}

	var response struct {
		OK       bool `json:"ok"`
		Channels []struct {
			ID   string `json:"id"`
			Name string `json:"name"`
		} `json:"channels"`
	}

	err = json.Unmarshal(body, &response)
	if err != nil {
		return "", err
	}

	for _, channel := range response.Channels {
		if channel.Name == groupName {
			return channel.ID, nil
		}
	}

	return "", fmt.Errorf("Group not found", string(body), err)
}

func isUserInGroup(apiToken, userID, groupID string) (bool, error) {
	url := "https://slack.com/api/conversations.members"

	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return false, err
	}

	req.Header.Add("Authorization", "Bearer "+apiToken) // Add the token as Bearer token in the Authorization header

	queryParams := req.URL.Query()
	queryParams.Add("channel", groupID)
	req.URL.RawQuery = queryParams.Encode()

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return false, err
	}
	defer resp.Body.Close()

	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return false, err
	}

	var response struct {
		OK      bool     `json:"ok"`
		Members []string `json:"members"`
	}

	err = json.Unmarshal(body, &response)
	if err != nil {
		return false, err
	}

	for _, member := range response.Members {
		if member == userID {
			return true, nil
		}
	}

	return false, nil
}

func VerifySlackRequest(req *http.Request) (bool, error) {
	// Read request body
	body, err := ioutil.ReadAll(req.Body)
	fmt.Println(body)
	if err != nil {
		return false, err
	}
	// Read file with signing secret
	file, err := os.Open("signing-secret.txt")
	if err != nil {
		log.Fatal(err)
	}
	defer file.Close()
	//Open scanner
	scanner := bufio.NewScanner(file)
	//initialise the signing secret variable
	signingsecret := "blank"
	// Iterate through each line of the file
	for scanner.Scan() {
		line := scanner.Text()
		fmt.Println(scanner.Text())
		signingsecret = line
	}

	// Check for any scanning errors
	if err := scanner.Err(); err != nil {
		log.Fatal(err)
	}

	// Reset request body to original state
	req.Body = ioutil.NopCloser(strings.NewReader(string(body)))

	// Retrieve Slack signature and timestamp from headers
	signature := req.Header.Get("X-Slack-Signature")
	timestamp := req.Header.Get("X-Slack-Request-Timestamp")

	// Concatenate timestamp and request body
	raw := fmt.Sprintf("v0:%s:%s", timestamp, string(body))
	fmt.Println(raw)

	// Compute HMAC-SHA256 hash with Slack signing secret
	hash := hmac.New(sha256.New, []byte(signingsecret))
	hash.Write([]byte(raw))
	expectedSignature := fmt.Sprintf("v0=%s", hex.EncodeToString(hash.Sum(nil)))

	// Compare expected signature with received signature
	return hmac.Equal([]byte(signature), []byte(expectedSignature)), nil
}

func (fe *frontendServer) userlookupresponse(w http.ResponseWriter, r *http.Request) {
	ctx1 := context.Background()
	id := mux.Vars(r)["id"]
	target := "http://userlookup:5003/user_lookup?user_id="
	httprequest, err := otelhttp.Get(ctx1, target+id)
	if err != nil {
		fmt.Fprint(w, err)
	}
	body, err := ioutil.ReadAll(httprequest.Body)
	if err != nil {
		fmt.Fprint(w, err)
	}
	message := string(body)
	//message := "Hello"
	fmt.Fprint(w, message)
}

// formatRequest generates ascii representation of a request
func formatRequest(r *http.Request) string {
	// Create return string
	var request []string
	// Add the request string
	url := fmt.Sprintf("%v %v %v", r.Method, r.URL, r.Proto)
	request = append(request, url)
	// Add the host
	request = append(request, fmt.Sprintf("Host : %v", r.Host))
	// Loop through headers
	for name, headers := range r.Header {
		name = strings.ToLower(name)
		for _, h := range headers {
			request = append(request, fmt.Sprintf("%v: %v", name, h))
		}
	}

	// If this is a POST, add post data
	if r.Method == "POST" {
		r.ParseForm()
		request = append(request, "\n")
		request = append(request, r.Form.Encode())
	}
	// Return the request as a string
	return strings.Join(request, "\n")
}
