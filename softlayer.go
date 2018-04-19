package main

import (
	"os"
	"strconv"
	"time"

	"github.com/gobuffalo/uuid"
	log "github.com/sirupsen/logrus"
	"github.com/softlayer/softlayer-go/filter"
	"github.com/softlayer/softlayer-go/services"
	"github.com/softlayer/softlayer-go/session"

	spec "github.com/hyeoncheon/honcheonui-spec"
)

// constants
const (
	ProviderName = "softlayer"
	APIEndPoint  = "https://api.softlayer.com/rest/3.1"
)

var logger *log.Entry

func init() {
	log.SetFormatter(&log.JSONFormatter{})
	log.SetOutput(os.Stdout)
	log.SetLevel(log.InfoLevel)
	logger = log.WithFields(log.Fields{
		"facility": "plugin",
		"program":  "softlayer",
	})
}

// plugin function holder
type softlayerProvider int8

// Init implements plugins.Provider
func (p softlayerProvider) Init() error {
	return nil
}

// CheckAccount implements plugins.Provider
func (p softlayerProvider) CheckAccount(user, pass string) (int, int, error) {
	sess := session.New(user, pass, APIEndPoint)
	service := services.GetAccountService(sess)
	data, err := service.
		Mask("id;accountId;parentId;companyName;email;firstName;lastName;ticketCount;openTicketCount;hardwareCount;virtualGuestCount").
		GetCurrentUser()
	if err != nil {
		logger.Errorf("softlayer api exception: %v", err)
		return 0, 0, err
	}

	logger.Debugf("account '%v' confirmed", *data.AccountId)
	return *data.Id, *data.AccountId, nil
}

// GetResources implements plugins.Provider
func (p softlayerProvider) GetResources(user, pass string) ([]interface{}, error) {
	sess := session.New(user, pass, APIEndPoint)
	service := services.GetAccountService(sess)
	data, err := service.
		Mask("id;uuid;accountId;users.id;hourlyBillingFlag;hostname;domain;notes;tagReferences.tag.name;provisionDate;bandwidthAllocation;privateNetworkOnlyFlag;primaryIpAddress;primaryBackendIpAddress;location.pathString;startCpus;maxCpu;maxCpuUnits;maxMemory;type.name;createDate;modifyDate;status.name;powerState.name;id;id;networkVlans.id;operatingSystem.id;datacenter.id;location.id;virtualRackName;pendingMigrationFlag;dedicatedAccountHostOnlyFlag;dedicatedHost;host").
		GetVirtualGuests()
	if err != nil {
		logger.Debugf("softlayer api exception: %v", err)
		return nil, err
	}

	var resources []interface{}
	for _, d := range data {
		cr := spec.HoncheonuiResource{
			Provider:           ProviderName,
			Type:               "vm",
			OriginalID:         strconv.Itoa(slInt(d.Id)),
			Name:               slString(d.Hostname) + "." + slString(d.Domain),
			Notes:              slString(d.Notes),
			GroupID:            strconv.Itoa(slInt(d.AccountId)),
			ResourceCreatedAt:  d.CreateDate.Time,
			ResourceModifiedAt: d.ModifyDate.Time,
			IPAddress:          slString(d.PrimaryIpAddress),
			Location:           slString(d.Location.PathString),
		}
		if d.Uuid != nil {
			cr.UUID, _ = uuid.FromString(*d.Uuid)
		}
		if slString(d.Status.Name) == "Active" {
			cr.IsConn = true
		}
		if slString(d.PowerState.Name) == "Running" {
			cr.IsOn = true
		}
		cr.Attributes = map[string]string{
			"MaxCpu":    strconv.Itoa(slInt(d.MaxCpu)),
			"MaxMemory": strconv.Itoa(slInt(d.MaxMemory)),
		}
		// this type of attributes are not used currently
		cr.IntegerAttributes = map[string]int{
			"MaxCpu":    slInt(d.MaxCpu),
			"MaxMemory": slInt(d.MaxMemory),
		}
		for _, t := range d.TagReferences {
			cr.Tags = append(cr.Tags, slString(t.Tag.Name))
		}
		for _, u := range d.Users {
			cr.UserIDs = append(cr.UserIDs, strconv.Itoa(*u.Id))
		}
		resources = append(resources, cr)
	}

	logger.Debugf("got %v virtual guests", len(resources))
	return resources, nil
}

// GetStatuses implements plugins.Provider
func (p softlayerProvider) GetStatuses(user, pass string) ([]interface{}, error) {
	sess := session.New(user, pass, APIEndPoint)
	service := services.GetAccountService(sess)
	data, err := service.
		Mask("id;uuid;status.name;powerState.name;modifyDate").
		GetVirtualGuests()
	if err != nil {
		logger.Debugf("softlayer api exception: %v", err)
		return nil, err
	}

	var resources []interface{}
	for _, d := range data {
		crs := spec.HoncheonuiStatus{
			OriginalID: strconv.Itoa(slInt(d.Id)),
		}
		if slString(d.Status.Name) == "Active" {
			crs.IsConn = true
		}
		if slString(d.PowerState.Name) == "Running" {
			crs.IsOn = true
		}
		resources = append(resources, crs)
	}

	logger.Debugf("got %v virtual guest statuses", len(resources))
	return resources, nil
}

// GetNotification implements plugins.Provider
func (p softlayerProvider) GetNotifications(user, pass string, from time.Time) ([]interface{}, error) {
	loc, _ := time.LoadLocation("US/Central")
	from = from.In(loc)
	return getTickets(user, pass, from)
}

const (
	ticketMaskType1 = "id;accountId;assignedUserId;group.name;subject.name;status.name;firstUpdate.editorType;firstUpdate.editorId;firstUpdate.entry;priority;title;createDate;modifyDate;lastEditDate;lastEditType;attachedVirtualGuests.id;attachedVirtualGuests.hostname;attachedVirtualGuests.domain;attachedVirtualGuests.users.id;attachedHardware;attachedResources"
)

func getTickets(user, pass string, from time.Time) ([]interface{}, error) {
	fromString := from.Format("01/02/2006 15:04:05")
	sess := session.New(user, pass, APIEndPoint)
	service := services.GetAccountService(sess)
	data, err := service.
		Mask(ticketMaskType1).
		Filter(filter.Build(
			filter.Path("tickets.group.name").NotContains("Sales"),
			filter.Path("tickets.firstUpdate.editorType").In("AUTO", "EMPLOYEE"),
			filter.Path("tickets.createDate").DateAfter(fromString),
		)).
		Limit(100).
		GetTickets()
	if err != nil {
		logger.Debugf("softlayer api exception: %v", err)
		return nil, err
	}

	var tickets []interface{}
	for _, d := range data {
		ct := spec.HoncheonuiNotification{
			Provider:   ProviderName,
			Type:       "ticket",
			OriginalID: strconv.Itoa(slInt(d.Id)),
			GroupID:    strconv.Itoa(slInt(d.AccountId)),
			UserID:     strconv.Itoa(slInt(d.AssignedUserId)),
			Title:      slString(d.Title),
			Content:    slString(d.FirstUpdate.Entry),
			IssuedAt:   d.CreateDate.Time,
			ModifiedAt: d.ModifyDate.Time,
			IssuedBy:   slString(d.FirstUpdate.EditorType),
		}

		if d.Group != nil {
			ct.Category = slString(d.Group.Name)
		}
		if d.Subject != nil {
			ct.Category += "/" + slString(d.Subject.Name)
		}
		if slString(d.Status.Name) != "Closed" {
			ct.IsOpen = true
		}
		for _, r := range d.AttachedResources {
			ct.ResourceIDs = append(ct.ResourceIDs, strconv.Itoa(slInt(r.AttachmentId)))
		}
		if d.AttachedVirtualGuests != nil {
			for _, vsi := range d.AttachedVirtualGuests {
				for _, u := range vsi.Users {
					ct.UserIDs = append(ct.UserIDs, strconv.Itoa(slInt(u.Id)))
				}
			}
		}
		tickets = append(tickets, ct)
	}
	return tickets, nil
}

// currently not used
func (p softlayerProvider) ParseEvent() (interface{}, error) {
	logger.Debugf("initialize softlayer...")
	ret := interface{}(map[string]string{"k1": "v1", "k2": "v2"})
	return ret, nil
}

//*** utilities for handling softlayer data

func slInt(i *int) int {
	if i != nil {
		return *i
	}
	return -1
}

func slString(s *string) string {
	if s != nil {
		return *s
	}
	return ""
}

// dummy function to satisfying package requirement
func main() {}

// Provider is exported name of softlayerProvider for honcheonui
var Provider softlayerProvider
