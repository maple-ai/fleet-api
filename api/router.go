package api

import (
	"net/http"

	"github.com/gorilla/mux"
	"github.com/maple-ai/syrup"
)

func Routes() syrup.Router {
	r := syrup.New(mux.NewRouter(), syrup.CorsMiddleware)
	r.NotFoundHandler = http.HandlerFunc(syrup.NotFoundHandler)
	r.Get("/ping", syrup.PongHandler)
	r.Post("/contact", contactUs)

	// Login
	func(api syrup.Router) {
		api.Post("/password", login)
		api.Post("/register", signup)
		api.Post("/google", googleSignin)
		api.Post("/forgot", sendReset)
		api.Post("/reset", doReset)
	}(r.Group("/auth"))

	// 'Logged in' middleware
	r.Use(secureMiddleware)

	// Admin/Supervisor API
	adminRouter(r.Group("/admin"))

	func(api syrup.Router) {
		api.Get("", getUser)
		api.Put("/profile", userMembershipMiddleware, updateUserProfile)
		api.Post("/password", updateUserPassword)

		// Get user privileges
		api.Get("/privileges", getUserPrivileges)

		// User membership apis
		api.Get("/membership", userMembershipMiddleware, getUserMembership)
		// Save membership
		api.Post("/membership", userMembershipMiddleware, updateUserMembershipDetails)
		// Submit membership
		api.Post("/membership/submit", userMembershipMiddleware, submitMembership)

		// Get billing details
		api.Get("/billing", getUserBilling)
		// Update payment method
		api.Post("/billing/card", addUserCard)

		// Get driver license info
		api.Get("/license", getUserDrivingLicenseInfo)
		// Get driver license photo
		api.Get("/license/{license_type}", getUserDrivingLicensePicture)
		// Upload driver license photo
		api.Post("/license/{license_type}", uploadUserDrivingLicense)
	}(r.Group("/user"))

	// Get garages (note: uses admin api call)
	r.Get("/garages", adminGetGarages)
	r.Get("/shift_settings", adminGetSettings)

	r.Use(userMembershipMiddleware, userMustMembershipMiddleware)

	func(api syrup.Router) {
		api.Get("", getShifts)
		api.Post("", createShift)

		api.Get("/search", shiftSearch)
		api.Delete("/{shift_id}", shiftMiddleware, cancelShift)
		api.Get("/history", getShiftHistory)
	}(r.Group("/shifts"))

	return r
}

func adminRouter(api syrup.Router) {
	// Must be supervisor
	// ** readable ** APIs here for supervisor
	api.Use(supervisorMiddleware)

	// Bikes
	api.Get("/bikes", adminGetBikes)
	api.Get("/bikes/maintenance", adminGetBikesNeedMaintenance)
	api.Get("/bikes/shift_maintenance", adminGetBikesNeedShiftMaintenance)
	func(api syrup.Router) {
		api.Get("", adminGetBike)
		api.Get("/operator-notes", adminGetBikeOperatorNotes)

		api.Get("/maintenance", adminGetBikeMaintenance)
		api.Post("/maintenance", adminSetBikeMaintenance)
		api.Post("/maintenance/{maintenance_log_id}/attachment", adminSetMaintenanceAttachment)
		api.Get("/maintenance/{maintenance_log_id}/attachment", adminGetMaintenanceAttachment)
		api.Delete("/maintenance/{maintenance_log_id}/attachment", adminDeleteMaintenanceAttachment)
		api.Put("/maintenance/{maintenance_log_id}", adminSetBikeMaintenance)
		api.Delete("/maintenance/{maintenance_log_id}", adminDeleteBikeMaintenance)
	}(api.Group("/bikes/{bike_id}", adminBikeMiddleware))

	// Garages
	api.Get("/garages", adminGetGarages)
	api.Get("/garages/{garage_id}", adminGarageMiddleware, adminGetGarage)

	// Users
	api.Get("/users", adminGetUsers)
	api.Get("/admins", adminGetAdminUsers)

	// Shift Calendar
	api.Get("/calendar", adminGetShiftCalendar)
	// Shift API
	api.Get("/shifts/{shift_id}", adminGetShiftInfo)

	// Shift shit: check in/out/reset & shift notes
	api.Post("/shifts/{shift_id}/check-in", adminShiftCheckIn)
	api.Post("/shifts/{shift_id}/check-out", adminShiftCheckOut)
	api.Post("/shifts/{shift_id}/reset", adminShiftReset)
	api.Post("/shifts/{shift_id}/notes", adminShiftNotes)
	api.Get("/shifts/{shift_id}/positions", adminGetGPSPositions)
	api.Get("/shifts/{shift_id}/operator-notes", adminGetShiftOperatorNotes)
	api.Post("/shifts/{shift_id}/operator-notes", adminSetShiftOperatorNotes)
	api.Post("/shifts/{shift_id}/status", adminApproveShiftStatus)
	api.Delete("/shifts/{shift_id}/status", adminApproveShiftStatus)
	api.Post("/shifts/{shift_id}/reassign/{bike_id}", adminReassignBike)

	api.Get("/payroll", adminPayroll)

	// User API
	func(api syrup.Router) {
		api.Get("", adminGetUser)
		// Get membership details
		api.Get("/membership", adminGetUserMembership)
		// Get driver license picture
		api.Get("/membership/license/{license_type}", adminGetUserDriverLicense)
		// Get payment details from Stripe
		api.Get("/payment", adminGetUserPaymentInformation)

		// Get user shifts
		api.Get("/shifts", getShifts)
		api.Get("/shifts/search", shiftSearch)
		api.Get("/shifts/history", getShiftHistory)

		// Restrict to admins below here (editing & deleting)
		api.Use(adminMiddleware)

		// Update user profile (TODO: needs work)
		api.Put("", adminSaveUser)
		// Nuclear-kind delete
		api.Delete("", adminDeleteUser)

		// Upload licenses
		api.Post("/license/{license_type}", uploadUserDrivingLicense)
		api.Delete("/license/{license_type}", deleteUserDrivingLicense)

		// ** membership stuff **

		// Update membership details
		api.Post("/membership", adminUpdateUserMembership)
		// Set Interview Date
		api.Post("/membership/interview", adminSetInterviewDate)
		// Set rating
		api.Post("/membership/rating", adminSetRating)
		// Accept mmebership (sets membership number)
		api.Post("/membership/accept", adminAcceptMembership)
		// Remove (reject) membership
		api.Delete("/membership", adminRejectMembership)
		// Move back to onboarding
		api.Delete("/membership/onboarding", adminMoveMembership)

		// Create a shift on behalf of user
		api.Post("/shifts", createShift)
		// Delete shift for user
		api.Delete("/shifts/{shift_id}", shiftMiddleware, cancelShift)

		// Set password
		api.Post("/password", adminUserSetPassword)
		// Set Email
		api.Post("/email", adminSetUserEmail)
		// Set Name
		api.Post("/name", adminSetUserName)
		// User privileges
		api.Put("/privileges", adminUserSetPrivileges)
		// Block/unblock user
		api.Post("/block", blockUser)
		api.Delete("/block", blockUser)
	}(api.Group("/users/{user_id}", adminUserMiddleware))

	// Must be Admin
	// ** editable ** APIs here for admins
	api.Use(adminMiddleware)

	api.Get("/settings", adminGetSettings)
	api.Post("/settings", adminSetSettings)

	// Bikes
	api.Post("/bikes", adminSaveBike)
	api.Put("/bikes/{bike_id}", adminBikeMiddleware, adminSaveBike)
	api.Post("/bikes/{bike_id}/archive", adminBikeMiddleware, adminDeleteBike)

	// Garages
	api.Post("/garages", adminSaveGarage)
	api.Put("/garages/{garage_id}", adminGarageMiddleware, adminSaveGarage)
	api.Delete("/garages/{garage_id}", adminGarageMiddleware, adminDeleteGarage)

	// Memberships API
	api.Get("/memberships", adminGetMemberships)
	api.Get("/memberships/stats", adminGetMembershipStats)

	api.Post("/payroll/payout", adminPayout)
}
