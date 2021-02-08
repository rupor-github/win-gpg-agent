cmake_minimum_required(VERSION 3.0)

message(STATUS "Running cmake version ${CMAKE_VERSION}")
if (NOT CMAKE_TOOLCHAIN_FILE)
    set(CMAKE_TOOLCHAIN_FILE "../cmake/win64.toolchain" CACHE STRING "default toolchain" FORCE)
endif()
if (NOT CMAKE_BUILD_TYPE)
    set(CMAKE_BUILD_TYPE "Release" CACHE STRING "default to release build type" FORCE)
endif()

project(win-gpg-agent NONE)
set(CMAKE_MODULE_PATH ${CMAKE_MODULE_PATH} ${CMAKE_SOURCE_DIR}/cmake)

message(STATUS "Build type: ${CMAKE_BUILD_TYPE}")
message(STATUS "Generator: ${CMAKE_GENERATOR}")

# Release installation directory
if(NOT CMAKE_HOST_SYSTEM_NAME STREQUAL "Linux" OR NOT CMAKE_CROSSCOMPILING)
    message(FATAL_ERROR "Currently unsuppored host environment \"${CMAKE_HOST_SYSTEM_NAME}\". CMake willl exit now.")
else()
    set(HOST_GOOS linux)
    set(HOST_GOARCH amd64)
endif()

if(MSYSTEM_NAME STREQUAL "win32")
    set(CROSS_GOOS windows)
    set(CROSS_GOARCH 386)
    message(STATUS "Targeting 32 bits Windows executables")
elseif(MSYSTEM_NAME STREQUAL "win64")
    set(CROSS_GOOS windows)
    set(CROSS_GOARCH amd64)
    message(STATUS "Targeting 64 bits Windows executables")
else()
    message(FATAL_ERROR "Currently unsuppored environment \"${MSYSTEM_NAME}\". CMake willl exit now.")
endif()

set(DEST_DIR "${PROJECT_SOURCE_DIR}/bin")
message(STATUS "Installation directory: ${DEST_DIR}")

find_package(Go 1.15 REQUIRED)
find_package(Git REQUIRED)

# Make sure we setup go properly
if (DEFINED ENV{GOPATH})
    set(GO_PATH "$ENV{GOPATH}")
else()
    set(GO_PATH "/tmp/gopkg-${CMAKE_PROJECT_NAME}")
    execute_process(COMMAND ${CMAKE_COMMAND} -E make_directory ${GO_PATH})
endif()
message(STATUS "Using go modules with GOPATH=${GO_PATH}")
if(GO_PATH MATCHES "^${PROJECT_SOURCE_DIR}.*")
    message(FATAL_ERROR "We are working in \"go modules outside of GOPATH mode\" - GOPATH should be outside of project source directory!")
endif()

########################################################################################################
# Setting up versioning
########################################################################################################

# Project version number
set(PRJ_VERSION_Major "1")
set(PRJ_VERSION_Minor "2")
set(PRJ_VERSION_Patch "1")

if (EXISTS "${PROJECT_SOURCE_DIR}/.git" AND IS_DIRECTORY "${PROJECT_SOURCE_DIR}/.git")
    execute_process(COMMAND ${CMAKE_SOURCE_DIR}/cmake/githash.sh ${GIT_EXECUTABLE}
        WORKING_DIRECTORY "${PROJECT_SOURCE_DIR}"
        RESULT_VARIABLE res
        OUTPUT_VARIABLE GIT_HASH
        ERROR_QUIET
        OUTPUT_STRIP_TRAILING_WHITESPACE)
    # This will cause reconfiguration on every commit
    set_property(DIRECTORY APPEND PROPERTY CMAKE_CONFIGURE_DEPENDS "${PROJECT_SOURCE_DIR}/.git/index")
else()
    set(GIT_HASH "no-git")
endif()
configure_file("${PROJECT_SOURCE_DIR}/cmake/version.go.in" "${PROJECT_SOURCE_DIR}/misc/version.go")
configure_file("${PROJECT_SOURCE_DIR}/cmake/agent.rc.in" "${PROJECT_SOURCE_DIR}/cmd/agent/resources.rc")
configure_file("${PROJECT_SOURCE_DIR}/cmake/agent.xml.in" "${PROJECT_SOURCE_DIR}/cmd/agent/manifest.xml")
configure_file("${PROJECT_SOURCE_DIR}/cmake/pinentry.rc.in" "${PROJECT_SOURCE_DIR}/cmd/pinentry/resources.rc")
configure_file("${PROJECT_SOURCE_DIR}/cmake/pinentry.xml.in" "${PROJECT_SOURCE_DIR}/cmd/pinentry/manifest.xml")
configure_file("${PROJECT_SOURCE_DIR}/cmake/sorelay.rc.in" "${PROJECT_SOURCE_DIR}/cmd/sorelay/resources.rc")
configure_file("${PROJECT_SOURCE_DIR}/cmake/sorelay.xml.in" "${PROJECT_SOURCE_DIR}/cmd/sorelay/manifest.xml")

# distribute history
execute_process(COMMAND ${CMAKE_COMMAND} -E env ${GIT_EXECUTABLE} changelog --all --tag "v${PRJ_VERSION_Major}.${PRJ_VERSION_Minor}.${PRJ_VERSION_Patch}" --stdout OUTPUT_FILE ${PROJECT_BINARY_DIR}/changelog.txt ERROR_QUIET)
install(FILES ${PROJECT_BINARY_DIR}/changelog.txt DESTINATION ${DEST_DIR} CONFIGURATIONS Release)

message(STATUS "Building version \"${PRJ_VERSION_Major}.${PRJ_VERSION_Minor}.${PRJ_VERSION_Patch}\" git \"${GIT_HASH}\"")

if(CMAKE_BUILD_TYPE STREQUAL "Debug")
    # Create git pre-commit hook to check formatting before commit
    if(EXISTS "${PROJECT_SOURCE_DIR}/.git" AND IS_DIRECTORY "${PROJECT_SOURCE_DIR}/.git")
        execute_process(COMMAND ${CMAKE_COMMAND} -E copy ${PROJECT_SOURCE_DIR}/cmake/pre-commit ${PROJECT_SOURCE_DIR}/.git/hooks/. WORKING_DIRECTORY ${PROJECT_SOURCE_DIR})
    endif()
endif()

########################################################################################################
# Some contexts dependent settings
########################################################################################################

if(CMAKE_BUILD_TYPE STREQUAL "Debug")
    set(TEST_RUN_ARGS "-v")
else()
    set(GO_MOD_ARGS "-mod=vendor")
endif()

########################################################################################################
# main target - always out of date, "go build" will figure it out
########################################################################################################

add_custom_target(bin_agent ALL
    DEPENDS ${PROJECT_SOURCE_DIR}/cmd/agent/resources.syso
    COMMAND GOPATH=${GO_PATH} GO111MODULE=on GOOS=${CROSS_GOOS} GOARCH=${CROSS_GOARCH}
        ${GO_EXECUTABLE} build -trimpath -o ${PROJECT_BINARY_DIR}/agent-gui${CMAKE_EXECUTABLE_SUFFIX}
        -ldflags='-H=windowsgui'
        ${GO_MOD_ARGS}
        ./cmd/agent
    COMMENT "Building agent-gui..."
    WORKING_DIRECTORY "${PROJECT_SOURCE_DIR}")
install(PROGRAMS ${PROJECT_BINARY_DIR}/agent-gui${CMAKE_EXECUTABLE_SUFFIX} DESTINATION ${DEST_DIR} CONFIGURATIONS Release)

add_custom_command(OUTPUT ${PROJECT_SOURCE_DIR}/cmd/agent/resources.syso
     DEPENDS ${PROJECT_SOURCE_DIR}/cmd/agent/resources.rc
         ${PROJECT_SOURCE_DIR}/cmd/agent/manifest.xml
         ${PROJECT_SOURCE_DIR}/cmd/agent/icon.ico
     COMMAND ${CMAKE_RC_COMPILER} -O coff
         -o ${PROJECT_SOURCE_DIR}/cmd/agent/resources.syso
         -i ${PROJECT_SOURCE_DIR}/cmd/agent/resources.rc
     WORKING_DIRECTORY "${PROJECT_SOURCE_DIR}"
     COMMENT "Building agent-gui resources...")

add_custom_target(bin_pinentry ALL
    DEPENDS ${PROJECT_SOURCE_DIR}/cmd/pinentry/resources.syso
    COMMAND GOPATH=${GO_PATH} GO111MODULE=on GOOS=${CROSS_GOOS} GOARCH=${CROSS_GOARCH}
        ${GO_EXECUTABLE} build -trimpath -o ${PROJECT_BINARY_DIR}/pinentry${CMAKE_EXECUTABLE_SUFFIX}
        ${GO_MOD_ARGS}
        ./cmd/pinentry
    COMMENT "Building pinentry..."
    WORKING_DIRECTORY "${PROJECT_SOURCE_DIR}")
install(PROGRAMS ${PROJECT_BINARY_DIR}/pinentry${CMAKE_EXECUTABLE_SUFFIX} DESTINATION ${DEST_DIR} CONFIGURATIONS Release)

add_custom_command(OUTPUT ${PROJECT_SOURCE_DIR}/cmd/pinentry/resources.syso
    DEPENDS ${PROJECT_SOURCE_DIR}/cmd/pinentry/resources.rc
        ${PROJECT_SOURCE_DIR}/cmd/pinentry/manifest.xml
        ${PROJECT_SOURCE_DIR}/cmd/pinentry/icon.ico
     COMMAND ${CMAKE_RC_COMPILER} -O coff
         -o ${PROJECT_SOURCE_DIR}/cmd/pinentry/resources.syso
         -i ${PROJECT_SOURCE_DIR}/cmd/pinentry/resources.rc
    WORKING_DIRECTORY "${PROJECT_SOURCE_DIR}"
    COMMENT "Building pinentry resources...")

add_custom_target(bin_sorelay ALL
    DEPENDS ${PROJECT_SOURCE_DIR}/cmd/sorelay/resources.syso
    COMMAND GOPATH=${GO_PATH} GO111MODULE=on GOOS=${CROSS_GOOS} GOARCH=${CROSS_GOARCH}
        ${GO_EXECUTABLE} build -trimpath -o ${PROJECT_BINARY_DIR}/sorelay${CMAKE_EXECUTABLE_SUFFIX}
        ${GO_MOD_ARGS}
        ./cmd/sorelay
    COMMENT "Building sorelay..."
    WORKING_DIRECTORY "${PROJECT_SOURCE_DIR}")
install(PROGRAMS ${PROJECT_BINARY_DIR}/sorelay${CMAKE_EXECUTABLE_SUFFIX} DESTINATION ${DEST_DIR} CONFIGURATIONS Release)

add_custom_command(OUTPUT ${PROJECT_SOURCE_DIR}/cmd/sorelay/resources.syso
    DEPENDS ${PROJECT_SOURCE_DIR}/cmd/sorelay/resources.rc
        ${PROJECT_SOURCE_DIR}/cmd/sorelay/manifest.xml
        ${PROJECT_SOURCE_DIR}/cmd/sorelay/icon.ico
     COMMAND ${CMAKE_RC_COMPILER} -O coff
         -o ${PROJECT_SOURCE_DIR}/cmd/sorelay/resources.syso
         -i ${PROJECT_SOURCE_DIR}/cmd/sorelay/resources.rc
    WORKING_DIRECTORY "${PROJECT_SOURCE_DIR}"
    COMMENT "Building sorelay resources...")

########################################################################################################
# Development
########################################################################################################

if(CMAKE_BUILD_TYPE STREQUAL "Debug")

    ########################################################################################################
    # Go linter
    ########################################################################################################

    # Running linters on main target
    find_program(GO_LINTER golangci-lint PATHS ${PROJECT_BINARY_DIR} NO_DEFAULT_PATH)
    if(NOT GO_LINTER)
        set(golangci-lint-ver "1.33.0")
        message(STATUS "Preparing golangci-lint ${golangci-lint-ver}")
        file(DOWNLOAD "https://github.com/golangci/golangci-lint/releases/download/v${golangci-lint-ver}/golangci-lint-${golangci-lint-ver}-linux-amd64.tar.gz" ${PROJECT_BINARY_DIR}/golangci-lint.tar.gz
            INACTIVITY_TIMEOUT 60
            TIMEOUT 300
            STATUS DOWNLOAD_RES
            EXPECTED_HASH SHA256=e2d6082f1df53c5d2c280765000f9e82783ea909ba419c6c4e172936b076031e)
        list(GET DOWNLOAD_RES 0 RES)
        list(GET DOWNLOAD_RES 1 MSG)
        if(RES)
            message(FATAL_ERROR "${MSG} : ${RES}")
        endif()
        execute_process(COMMAND ${CMAKE_COMMAND} -E tar xf golangci-lint.tar.gz WORKING_DIRECTORY ${PROJECT_BINARY_DIR})
        execute_process(COMMAND ${CMAKE_COMMAND} -E copy golangci-lint-${golangci-lint-ver}-linux-amd64/golangci-lint golangci-lint WORKING_DIRECTORY ${PROJECT_BINARY_DIR})
        execute_process(COMMAND ${CMAKE_COMMAND} -E remove_directory golangci-lint-${golangci-lint-ver}-linux-amd64 WORKING_DIRECTORY ${PROJECT_BINARY_DIR})
        execute_process(COMMAND ${CMAKE_COMMAND} -E remove golangci-lint.tar.gz WORKING_DIRECTORY ${PROJECT_BINARY_DIR})
        set(GO_LINTER ${PROJECT_BINARY_DIR}/golangci-lint)
    endif()

    add_custom_target(lint
        COMMAND GOPATH=${GO_PATH} GO111MODULE=on GOOS=${CROSS_GOOS} GOARCH=${CROSS_GOARCH} ${GO_LINTER} run
        COMMENT "Mega-linting project with ${GO_LINTER}..."
        WORKING_DIRECTORY "${PROJECT_SOURCE_DIR}")

    ########################################################################################################
    # Go dependency management
    ########################################################################################################

    add_custom_target(deps_tidy
        SOURCES ${PROJECT_SOURCE_DIR}/go.mod
        COMMAND GOPATH=${GO_PATH} GO111MODULE=on GOOS=${CROSS_GOOS} GOARCH=${CROSS_GOARCH} ${GO_EXECUTABLE} mod tidy
        WORKING_DIRECTORY "${PROJECT_SOURCE_DIR}"
        COMMENT "Cleaning dependencies (go mod tidy)"
        VERBATIM)

    add_custom_target(deps_vendor
        SOURCES ${PROJECT_SOURCE_DIR}/go.mod
        COMMAND GOPATH=${GO_PATH} GO111MODULE=on GOOS=${CROSS_GOOS} GOARCH=${CROSS_GOARCH} ${GO_EXECUTABLE} mod vendor
        WORKING_DIRECTORY "${PROJECT_SOURCE_DIR}"
        COMMENT "Creating vendor directory (go mod vendor)"
        VERBATIM)
endif()
