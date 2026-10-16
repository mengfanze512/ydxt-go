package api

import (
	"encoding/json"
	"fmt"
	"strings"
	"testing"
	"ydxt-go/internal/model"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

func checkoutTestDB(t *testing.T) *gorm.DB {
	t.Helper()
	db := deliveryTestDB(t)
	if err := db.AutoMigrate(&model.CartItem{}, &model.ShopGoods{}); err != nil {
		t.Fatal(err)
	}
	mustCreate(t, db, &model.ShopGoods{ID: 1, Title: "Test goods", Price: 1200, Stock: 5, Status: 1})
	mustCreate(t, db, &model.CartItem{ID: 1, UserID: 1, ItemType: "shop", GoodsID: 1, Quantity: 2})
	return db
}

func forbidUnpaidPaymentTime(t *testing.T, db *gorm.DB) {
	t.Helper()
	// SQLite trigger reproduces rejection of invalid unpaid datetime inserts.
	if err := db.Exec(`CREATE TRIGGER reject_unpaid_time BEFORE INSERT ON orders
		WHEN NEW.status = 'unpaid' AND NEW.pay_time IS NOT NULL
		BEGIN SELECT RAISE(ABORT, 'unpaid payment time must be omitted'); END`).Error; err != nil {
		t.Fatal(err)
	}
}

func TestCartCheckoutUnpaidTimeAndPayment(t *testing.T) {
	db := checkoutTestDB(t)
	forbidUnpaidPaymentTime(t, db)
	mustCreate(t, db, &model.CartItem{ID: 2, UserID: 2, ItemType: "shop", GoodsID: 1, Quantity: 1})
	c, rec := deliveryTestContext(t, 1, "/api/v1/shop/orders", gin.H{"cart_item_ids": []uint64{1, 1}})
	CreateShopOrder(c)
	if rec.Code != 200 {
		t.Fatal(rec.Body.String())
	}
	var response struct{ Data createShopOrderResponse }
	if err := json.Unmarshal(rec.Body.Bytes(), &response); err != nil || response.Data.Amount != 24 {
		t.Fatalf("incorrect checkout amount: %s", rec.Body.String())
	}
	var order model.Order
	if err := db.First(&order, response.Data.ID).Error; err != nil || !order.PayTime.IsZero() {
		t.Fatal("unpaid order has a payment timestamp")
	}
	var count int64
	db.Model(&model.OrderItem{}).Where("order_id = ?", order.ID).Count(&count)
	if count != 1 {
		t.Fatal("duplicate cart selection created duplicate details")
	}
	db.Model(&model.CartItem{}).Where("user_id = ?", 2).Count(&count)
	if count != 1 {
		t.Fatal("checkout removed another user's cart")
	}
	db.Model(&model.CartItem{}).Where("user_id = ?", 1).Count(&count)
	if count != 0 {
		t.Fatal("successful checkout did not remove selected cart")
	}
	c, rec = deliveryTestContext(t, 1, "/api/v1/shop/orders/1/pay", nil)
	c.Params = gin.Params{{Key: "id", Value: fmt.Sprint(order.ID)}}
	PayOrder(c)
	if rec.Code != 200 {
		t.Fatal(rec.Body.String())
	}
	if err := db.First(&order, order.ID).Error; err != nil || order.Status != "paid" || order.PayTime.IsZero() {
		t.Fatal("payment did not write a real payment timestamp")
	}
}

func TestCartCheckoutFailuresKeepCartAndExplainStage(t *testing.T) {
	for _, test := range []struct{ Name, SQL, Reason string }{
		{"main", "DROP TABLE orders", "写入订单主表 orders 失败"},
		{"details", "DROP TABLE order_items", "写入订单明细 order_items 失败"},
		{"cart_cleanup", `CREATE TRIGGER reject_cart_delete BEFORE DELETE ON cart_items
			BEGIN SELECT RAISE(ABORT, 'test cart delete failure'); END`, "清理已下单购物车 cart_items 失败"},
	} {
		t.Run(test.Name, func(t *testing.T) {
			db := checkoutTestDB(t)
			if err := db.Exec(test.SQL).Error; err != nil {
				t.Fatal(err)
			}
			c, rec := deliveryTestContext(t, 1, "/api/v1/shop/orders", gin.H{"cart_item_ids": []uint64{1}})
			CreateShopOrder(c)
			if rec.Code != 500 || !strings.Contains(rec.Body.String(), test.Reason) {
				t.Fatalf("failure reason hidden: %s", rec.Body.String())
			}
			var count int64
			db.Model(&model.CartItem{}).Where("id = ?", 1).Count(&count)
			if count != 1 {
				t.Fatal("failed checkout lost cart")
			}
			if test.Name != "main" {
				db.Model(&model.Order{}).Count(&count)
				if count != 0 {
					t.Fatal("failed checkout left a partial order")
				}
			}
		})
	}
}

func TestCourseCheckoutOmitsUnpaidTime(t *testing.T) {
	db := checkoutTestDB(t)
	forbidUnpaidPaymentTime(t, db)
	mustCreate(t, db, &model.Course{ID: 1, Title: "Test course", Level: "single", Price: 20, Status: 1})
	c, rec := deliveryTestContext(t, 1, "/api/v1/course-orders", gin.H{"course_id": 1})
	CreateCourseOrder(c)
	if rec.Code != 200 {
		t.Fatal(rec.Body.String())
	}
}
