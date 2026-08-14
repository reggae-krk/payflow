package users

import (
	"context"
	"log"
	"testing"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/reggae-krk/payflow/internal/users/testhelpers"
	"github.com/stretchr/testify/suite"
)

const FirstUserEmail string = "first@user.com"
const FirstUserChangedEmail string = "first-changed@user.com"

type UserRepoTestSuite struct {
	suite.Suite
	pgContainer *testhelpers.PostgresContainer
	repository  *userRepository
	ctx         context.Context
}

func (suite *UserRepoTestSuite) SetupSuite() {
	suite.ctx = context.Background()
	pgContainer, err := testhelpers.CreatePostgresContainer(suite.ctx)
	if err != nil {
		log.Fatal(err)
	}
	suite.pgContainer = pgContainer
	pool, err := pgxpool.New(suite.ctx, suite.pgContainer.ConnectionString)
	if err != nil {
		log.Fatal(err)
	}
	repository := NewUserRepository(pool)
	suite.repository = repository
}

func (suite *UserRepoTestSuite) TearDownSuite() {
    if suite.repository != nil && suite.repository.pool != nil {
        suite.repository.pool.Close()
    }
    if err := suite.pgContainer.Terminate(suite.ctx); err != nil {
        log.Fatalf("error terminating postgres container: %s", err)
    }
}

func (suite *UserRepoTestSuite) SetupTest() {
    _, err := suite.repository.pool.Exec(
        suite.ctx,
        "TRUNCATE TABLE users RESTART IDENTITY CASCADE",
    )
    if err != nil {
        log.Fatal(err)
    }
}

func TestUserRepoTestSuite(t *testing.T) {
	suite.Run(t, new(UserRepoTestSuite))
}

func (suite *UserRepoTestSuite) TestCreateUser() {
    user, err := suite.createUser(FirstUserEmail)

    suite.Require().NoError(err)
    suite.Require().NotNil(user)
    suite.Require().NotZero(user.Id)
}

func (suite *UserRepoTestSuite) TestGetUserById() {
    user, err := suite.createUser(FirstUserEmail)
    suite.Require().NoError(err)

    user, err = suite.repository.GetByID(suite.ctx, user.Id)
	
    suite.Require().NoError(err)
    suite.Require().NotNil(user)
    suite.Require().Equal(FirstUserEmail, user.Email)
}

func (suite *UserRepoTestSuite) TestGetUserByEmail() {
    user, err := suite.createUser(FirstUserEmail)
    suite.Require().NoError(err)

	user, err = suite.repository.GetByEmail(suite.ctx, FirstUserEmail)
	
    suite.Require().NoError(err)
    suite.Require().NotNil(user)
    suite.Require().Equal(FirstUserEmail, user.Email)
}

func (suite *UserRepoTestSuite) TestUpdateUserEmail() {
    user, err := suite.createUser(FirstUserEmail)
    suite.Require().NoError(err)

    err = suite.repository.UpdateEmail(suite.ctx, user.Id, FirstUserChangedEmail)

    suite.Require().NoError(err)
    
    user, err = suite.repository.GetByEmail(suite.ctx, FirstUserChangedEmail)

    suite.Require().NotNil(user)
    suite.Require().Equal(FirstUserChangedEmail, user.Email)
}

func (suite *UserRepoTestSuite) TestUpdateUserEmailByTakenValue() {
    user, err := suite.createUser(FirstUserEmail)
    suite.Require().NoError(err)

    user, err = suite.createUser(FirstUserChangedEmail)

    suite.Require().NoError(err)

	err = suite.repository.UpdateEmail(suite.ctx, user.Id, FirstUserChangedEmail)
	
    suite.Require().Error(err)
    suite.Require().ErrorIs(err, ErrEmailTaken)
}

func (suite *UserRepoTestSuite) TestDeleteUserById() {
    user, err := suite.createUser("delete@user.com")

    suite.Require().NoError(err)
    suite.Require().NotNil(user)
    suite.Require().NotZero(user.Id)

    id := user.Id
    email := user.Email
    err = suite.repository.DeleteByID(suite.ctx, id)
	
    suite.Require().NoError(err)

    user, err = suite.repository.GetByID(suite.ctx, id)

    suite.Require().Error(err)
	suite.Require().ErrorIs(err, ErrNoRows)
    suite.Require().Nil(user)

    user, err = suite.repository.GetByEmail(suite.ctx, email)

    suite.Require().Error(err)
	suite.Require().ErrorIs(err, ErrNoRows)
    suite.Require().Nil(user)
}

func (suite *UserRepoTestSuite) createUser(email string) (*User, error) {
    return suite.repository.CreateUser(suite.ctx, email, "hashS%24")
}