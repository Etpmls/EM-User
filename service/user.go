package service

import (
	"context"
	"encoding/json"
	"errors"
	"github.com/Etpmls/EM-Auth/src/application"
	"github.com/Etpmls/EM-Auth/src/application/protobuf"
	"github.com/Etpmls/EM-User/client"
	"github.com/Etpmls/EM-User/database"
	"github.com/Etpmls/EM-User/model"
	pb "github.com/Etpmls/EM-User/proto/pb"
	"github.com/Etpmls/Etpmls-Micro/v2/define"
	em_protobuf "github.com/Etpmls/Etpmls-Micro/v2/protobuf"
	em "github.com/Etpmls/Etpmls-Micro/v3"
	em_library "github.com/Etpmls/Etpmls-Micro/v3/library"
	"github.com/Etpmls/Etpmls-Micro/v3/proto/empb"
	"github.com/go-redis/redis/v8"
	"google.golang.org/grpc/codes"
	"gorm.io/gorm"
	"strconv"
	"strings"
)

type ServiceUser struct {
	pb.UnimplementedUserServer
}

// User Register
// 用户注册
func (this *ServiceUser) Register(ctx context.Context, request *pb.UserRegister) (*empb.Response, error) {
	return em.ErrorTranslate(ctx, codes.PermissionDenied, "ERROR_MESSAGE_RegistrationClosed", nil, nil)
}


// User Login
// 用户登录
type validateUserLogin struct {
	Username string `json:"username" validate:"required,max=255"`
	Password string `json:"password" validate:"required,max=255"`
}
func (this *ServiceUser) Login(ctx context.Context, request *pb.UserLogin) (*empb.Response, error)  {
	// Validate
	{
		var vd validateUserLogin
		err := em.Validator.Validate(request, &vd)
		if err != nil {
			em.LogWarn.FullPath(err.Error())
			return em.ErrorTranslate(ctx, codes.InvalidArgument, "ERROR_Validate", nil, err)
		}
	}

	// Verify Username & Password
	var us model.User
	usr, err := us.Verify(request.Username, request.Password)
	if err != nil {
		em.LogInfo.Path("Verify user failed!")
		return em.ErrorTranslate(ctx, codes.Unauthenticated, "ERROR_Login", nil, err)
	}

	//JWT
	token, err := us.UserGetToken(usr.ID, usr.Username)
	if err != nil {
		em.LogError.FullPath("Get Token failed! Error:" + err.Error())
		return em.ErrorTranslate(ctx, codes.Internal, "ERROR_Login", nil, err)
	}

	//Return Token
	resData := make(map[string]string)
	resData["token"] = token

	return em.SuccessTranslate(ctx, codes.OK, "SUCCESS_Login", resData)
}


// User Logout
// 用户登出
func (this *ServiceUser) Logout(ctx context.Context, request *em_protobuf.Empty) (*empb.Response, error) {
	return em.SuccessTranslate(ctx, codes.OK, "SUCCESS_Logout", nil)
}

// Get current user
// 获取当前用户
func (this *ServiceUser) GetCurrent(ctx context.Context, request *pb.UserGetCurrent) (*empb.Response, error) {
	e, _ := em.Kv.ReadKey(define.KvCacheEnable)
	if strings.ToLower(e) == "true" {
		return this.getCurrent_Cache(ctx, request, strings.ToLower(e) == "true")
	}

	return this.getCurrent_NoCache(ctx, request, strings.ToLower(e) == "true")
}
func (this *ServiceUser) getCurrent_NoCache(ctx context.Context, request *pb.UserGetCurrent, enableCache bool) (*empb.Response, error) {
	// Get User By request
	var user model.User
	u, err := user.GetUserByToken(request.Token)
	if err != nil {
		return em.ErrorTranslate(ctx, codes.InvalidArgument, "ERROR_GetUser", nil, err)
	}

	// Filter some field
	filter_user, err := user.InterfaceToUserGetOne(u)
	if err != nil {
		em.LogError.Path(err)
		return em.ErrorTranslate(ctx, codes.InvalidArgument, "ERROR_GetUser", nil, err)
	}

	// Ignore the avatar tag in the User structure
	type tmp struct {
		model.UserGetOne
		Avatar string     `json:"avatar"`
		Roles []string `json:"roles"`
	}
	var userApi = tmp{ UserGetOne: filter_user }

	// Avatar
	// 1.Get token By Request
	var path string
	path, _ = em.NewClient().User_GetAvatar(request.GetToken(), uint32(u.ID), application.Relationship_User_Avatar)


	userApi.Avatar = path
	// Roles
	var r []model.Role
	_ =database.DB.Model(&u).Association("Roles").Find(&r)
	for _, v := range r {
		userApi.Roles = append(userApi.Roles, v.Name)
	}

	if enableCache {
		b, err := json.Marshal(userApi)
		if err != nil {
			em.LogError.Path(err)
		} else {
			var m = make(map[string]string)
			m[strconv.Itoa(int(u.ID))] = string(b)
			em.Cache.SetHash(application.Cache_UserGetCurrent, m)
		}
	}


	return em.SuccessTranslate(ctx, codes.OK, "SUCCESS_GetUser", userApi)
}
func (this *ServiceUser) getCurrent_Cache(ctx context.Context, request *pb.UserGetCurrent, enableCache bool) (*empb.Response, error) {
	var user model.User
	id, err := user.GetUserIdByToken(request.Token)
	if err != nil {
		em.LogError.FullPath(err)
		return em.ErrorTranslate(ctx, codes.InvalidArgument, "ERROR_GetUser", nil, err)
	}

	str, err := em.Cache.GetHash(application.Cache_UserGetCurrent, strconv.Itoa(id))
	if err != nil {
		if err == redis.Nil {
			return this.getCurrent_NoCache(ctx, request, enableCache)
		}
		em.LogError.FullPath(err)
		return em.ErrorTranslate(ctx, codes.InvalidArgument, "ERROR_GetUser", nil, err)
	}

	type tmp struct {
		model.UserGetOne
		Avatar string     `json:"avatar"`
		Roles []string `json:"roles"`
	}
	var userApi tmp
	err = json.Unmarshal([]byte(str), &userApi)
	if err != nil {
		em.LogError.FullPath(err)
		em.Cache.DeleteHash(application.Cache_UserGetCurrent, strconv.Itoa(int(id)))
	}

	return em.SuccessTranslate(ctx, codes.OK, "SUCCESS_GetUser", userApi)
}

// Get all user
// 获取全部用户
func (this *ServiceUser) GetAll(ctx context.Context, request *em_protobuf.Pagination) (*empb.Response, error) {
	// 重写ApiUserGetAllV2的Roles字段，防止泄露隐私字段信息
	type Role model.RoleGetOne
	type User struct {
		model.UserGetOne
		Roles []Role `gorm:"many2many:role_users" json:"roles"`
	}
	var data []User

	// 获取分页和标题
	var orm em_library.Gorm
	limit, offset := orm.GeneratePaginationLimit(int(request.Number), int(request.Size))
	var count int64
	// Get the title of the search, if not get all the data
	// 获取搜索的标题，如果没有获取全部数据
	search := request.Search

	database.DB.Model(&User{}).Preload("Roles").Where("username " +database.FUZZY_SEARCH+ " ?", "%"+ search +"%").Count(&count).Limit(limit).Offset(offset).Find(&data)

	m := map[string]interface{}{application.FieldData: data, application.FieldCount: count}

	return em.SuccessTranslate(ctx, codes.OK,"SUCCESS_Get", m)
}

// Create user
// 创建用户
type validate_UserCreate struct {
	validateUserLogin
	Roles []model.Role `json:"roles" validate:"required"`
}
func (this *ServiceUser) Create(ctx context.Context, request *pb.UserCreate) (*empb.Response, error) {
	// Validate
	{
		var vd validate_UserCreate
		err := em.Validator.Validate(request, &vd)
		if err != nil {
			em.LogWarn.FullPath(err)
			return em.ErrorTranslate(ctx, codes.InvalidArgument, "ERROR_Validate", nil, err)
		}
	}

	// Request -> User
	var user model.User
	u, err := user.InterfaceToUser(request)
	if err != nil {
		return em.ErrorTranslate(ctx, codes.InvalidArgument, "ERROR_Create", nil, err)
	}

	// Check if Username exists
	// 检查Username是否存在
	var count_username int64
	database.DB.Model(&model.User{}).Where("username = ?", u.Username).Count(&count_username)
	if count_username != 0 {
		em.LogInfo.Path("Username already exists")
		return em.ErrorTranslate(ctx, codes.InvalidArgument, "ERROR_MESSAGE_DuplicateUserName", nil, errors.New("Username already exists"))
	}

	// Check if the role exists
	// 检查role是否存在
	var role_ids []uint
	for _, v := range u.Roles {
		role_ids = append(role_ids, v.ID)
	}
	var count int64
	database.DB.Model(&model.Role{}).Where("id IN ?", role_ids).Count(&count)
	if int(count) != len(role_ids) {
		em.LogError.FullPath("Role does not exist")
		return em.ErrorTranslate(ctx, codes.InvalidArgument, "ERROR_Create", nil, errors.New("Role does not exist"))
	}

	// Create User
	// 创建用户
	err = database.DB.Transaction(func(tx *gorm.DB) error {
		// Bcrypt Password
		u.Password, err = user.BcryptPassword(u.Password)
		if err != nil {
			return em.LogError.FullPathWithError("Password encryption failed" + err.Error())
		}

		// Create User
		result := tx.Create(&u)
		if result.Error != nil {
			return em.LogError.FullPathWithError("Create user failed" + result.Error.Error())
		}

		return nil
	})
	if err != nil {
		return em.ErrorTranslate(ctx, codes.InvalidArgument, "ERROR_Create", nil, err)
	}

	// Delete Cache
	// 删除缓存
	e, _ := em.Kv.ReadKey(define.KvCacheEnable)
	if strings.ToLower(e) == "true" {
		em.Cache.DeleteString(application.Cache_UserGetAll)
	}


	data, err := user.InterfaceToUserGetOne(u)
	if err != nil {
		// No need to return
		em.LogError.FullPath(err.Error())
	}
	return em.SuccessTranslate(ctx, codes.OK, "SUCCESS_Create", data)
}

// Edit user
// 编辑用户
type validate_UserEdit struct {
	ID uint             `json:"id" validate:"required"`
	Username string     `json:"username" validate:"required,max=255"`
	Roles []model.Role `json:"roles" validate:"required"`
}
func (this *ServiceUser) Edit(ctx context.Context, request *pb.UserEdit) (*empb.Response, error) {
	// Validate
	var vd validate_UserEdit
	err := em.ChangeType(request, &vd)
	if err != nil {
		em.LogError.Path(err.Error())
		return em.ErrorTranslate(ctx, codes.InvalidArgument, "ERROR_Edit", nil, err)
	}
	err = em.Validator.ValidateStruct(vd)
	if err != nil {
		return em.Error(codes.InvalidArgument, err.Error(, nil, err)
	}

	// Request -> User
	var user model.User
	u, err := user.InterfaceToUser(request)
	if err != nil {
		return em.ErrorTranslate(ctx, codes.InvalidArgument, "ERROR_Edit", nil, err)
	}

	// Find if the user exists
	// 查找该用户是否存在
	var form model.User
	result := database.DB.First(&form, request.Id)
	if result.RowsAffected == 0 {
		em.LogWarn.Path("No user record")
		return em.Error(codes.InvalidArgument, "No user record", nil, err)
	}

	// Check if Username exists
	// 检查Username是否存在
	var count_username int64
	database.DB.Model(&model.User{}).Where("username = ?", u.Username).Not(request.Id).Count(&count_username)
	if count_username != 0 {
		em.LogDebug.Path("The user name already exists!")
		return em.ErrorTranslate(ctx, codes.InvalidArgument, "ERROR_MESSAGE_DuplicateUserName", nil, err)
	}

	// Check if the role exists
	// 检查role是否存在
	var role_ids []uint
	for _, v := range u.Roles {
		role_ids = append(role_ids, v.ID)
	}
	var count int64
	database.DB.Model(&model.Role{}).Where("id IN ?", role_ids).Count(&count)
	if int(count) != len(role_ids) {
		em.LogWarn.Path("Role does not exist")
		return em.Error(codes.InvalidArgument, "Role does not exist", nil, err)
	}

	// If user set new password
	if len(u.Password) > 0 {
		var user model.User
		u.Password, err = user.BcryptPassword(u.Password)
		if err != nil {
			em.LogError.FullPath(err.Error())
			return em.ErrorTranslate(ctx, codes.InvalidArgument, "ERROR_Edit", nil, err)
		}
	}

	err = database.DB.Transaction(func(tx *gorm.DB) error {
		// Replace association
		// 替换关联
		var roleslist []model.Role
		for _, v := range u.Roles {
			roleslist = append(roleslist, model.Role{
				ID:          v.ID,
			})
		}
		err = tx.Model(&model.User{ID: u.ID}).Association("Roles").Replace(roleslist)
		if err != nil {
			return em.LogError.FullPathWithError(err.Error())
		}

		// Update operation, the updates method will not affect the association
		// 更新操作，updates方法不会影响关联
		result := tx.Model(&model.User{}).Where(u.ID).Updates(u)
		if result.Error != nil {
			return em.LogError.FullPathWithError(result.Error.Error())
		}

		return nil
	})
	if err != nil {
		return em.ErrorTranslate(ctx, codes.InvalidArgument, "ERROR_Edit", nil, err)
	}

	// Delete Cache
	// 删除缓存
	e, _ := em.Kv.ReadKey(define.KvCacheEnable)
	if strings.ToLower(e) == "true" {
		em.Cache.DeleteString(application.Cache_UserGetAll)
		em.Cache.DeleteHash(application.Cache_UserGetCurrent, strconv.Itoa(int(request.Id)))
	}


	data, err := user.InterfaceToUserGetOne(u)
	if err != nil {
		// No need to return
		em.LogError.FullPath(err.Error())
	}

	return em.SuccessTranslate(ctx, codes.OK,  "SUCCESS_Edit", data)
}

// Delete user
// 删除用户
type validate_UserDelete struct {
	Users []model.User `json:"users" validate:"required"`
}
func (this *ServiceUser) Delete(ctx context.Context, request *pb.UserDelete) (*empb.Response, error) {
	// Validate
	var vd validate_UserDelete
	err := em.ChangeType(request, &vd)
	if err != nil {
		em.LogError.FullPath(err.Error())
		return em.ErrorTranslate(ctx, codes.InvalidArgument, "ERROR_Delete", nil, err)
	}
	err = em.Validator.ValidateStruct(vd)
	if err != nil {
		return em.ErrorTranslate(ctx, codes.InvalidArgument, err.Error(, nil, err)
	}

	var ids []int
	for _, v := range request.Users {
		ids = append(ids, int(v.Id))
	}

	// Find if admin is included in ids
	// 查找ids中是否包含admin
	b := em.CheckIfSliceContainsInt(1, ids)
	if b {
		em.LogWarn.FullPath(("Cannot delete administrator"))
		return em.ErrorTranslate(ctx, codes.InvalidArgument, "ERROR_MESSAGE_ProhibitOperationOfAdministratorUsers", nil, errors.New("Cannot delete administrator"))
	}

	err = database.DB.Transaction(func(tx *gorm.DB) error {
		var u []model.User
		tx.Where("id IN ?", ids).Find(&u)

		// 删除用户
		result := tx.Delete(&u)
		if result.Error != nil {
			em.LogError.FullPath(result.Error.Error())
			return result.Error
		}

		// 删除关联
		err = tx.Model(&u).Association("Roles").Clear()
		if err != nil {
			em.LogError.FullPath(err.Error())
			return err
		}

		return nil
	})
	if err != nil {
		return em.ErrorTranslate(ctx, codes.InvalidArgument,  "ERROR_Delete", nil, err)
	}

	// Delete Cache
	// 删除缓存
	e, _ := em.Kv.ReadKey(define.KvCacheEnable)
	if strings.ToLower(e) == "true" {
		em.Cache.DeleteString(application.Cache_UserGetAll)
		var tmp []string
		for _, v := range ids {
			tmp = append(tmp, strconv.Itoa(int(v)))
		}
		em.Cache.DeleteHash(application.Cache_UserGetCurrent, strings.Join(tmp, " "))
	}


	return em.SuccessTranslate(ctx, codes.OK, "SUCCESS_Delete", nil)
}

// Update user information
// 更新用户信息
type validate_UserUpdateInformation struct {
	Password string           `json:"password" validate:"omitempty,min=6,max=50"`
	Avatar   model.Attachment `json:"avatar"`
}
func (this *ServiceUser) UpdateInformation(ctx context.Context, request *protobuf.UserUpdateInformation) (*empb.Response, error) {
	// Validate
	{
		err := em.Validator.Validate(request, &validate_UserUpdateInformation{})
		if err != nil {
			em.LogWarn.FullPath(err.Error())
			return em.ErrorTranslate(ctx, codes.InvalidArgument, "ERROR_Validate", nil, err)
		}
	}

	// Get User id
	var user model.User
	id, err := user.GetUserIdByRequest(ctx)
	if err != nil {
		return em.ErrorTranslate(ctx, codes.InvalidArgument, "ERROR_Update", nil, err)
	}

	// Request -> User
	u, err := user.InterfaceToUser(request)
	if err != nil {
		return em.ErrorTranslate(ctx, codes.InvalidArgument, "ERROR_Update", nil, err)
	}

	// Update
	err = database.DB.Transaction(func(tx *gorm.DB) error {

		// Create avatar attachment
		if len(request.GetAvatar().GetPath()) > 0 {
			err := client.NewClient().User_CreateAvatar(ctx, request.GetAvatar().GetPath(), uint32(id), application.Relationship_User_Avatar)
			if err != nil {
				return err
			}
		}

		// Update password if exists
		if len(u.Password) > 0 {
			u.Password, err = user.BcryptPassword(u.Password)
		}

		result := tx.Model(&model.User{ID: uint(id)}).Updates(&u)
		if result.Error != nil {
			em.LogError.FullPath(result.Error.Error())
			return result.Error
		}

		return nil
	})
	if err != nil {
		return em.ErrorTranslate(ctx, codes.InvalidArgument, "ERROR_Update", nil, err)
	}

	e, _ := em.Kv.ReadKey(define.KvCacheEnable)
	if strings.ToLower(e) == "true" {
		em.Cache.DeleteString(application.Cache_UserGetAll)
		em.Cache.DeleteHash(application.Cache_UserGetCurrent, strconv.Itoa(id))
	}

	return em.SuccessTranslate(ctx, codes.OK, "SUCCESS_Update", nil)
}
