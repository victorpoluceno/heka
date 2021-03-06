# This Source Code Form is subject to the terms of the Mozilla Public
# License, v. 2.0. If a copy of the MPL was not distributed with this
# file, You can obtain one at http://mozilla.org/MPL/2.0/.

cmake_minimum_required(VERSION 2.8.7 FATAL_ERROR)
project(heka C)
set(CPACK_PACKAGE_DESCRIPTION_SUMMARY "High performance data gathering, analysis, monitoring, and reporting.")
set(CPACK_PACKAGE_VERSION_MAJOR 0)
set(CPACK_PACKAGE_VERSION_MINOR 5)
set(CPACK_PACKAGE_VERSION_PATCH 0)
set(CPACK_PACKAGE_VENDOR "Mozilla")
set(CPACK_RESOURCE_FILE_LICENSE "${CMAKE_SOURCE_DIR}/LICENSE.txt")
set(CPACK_PACKAGE_CONTACT "heka@mozilla.org")
set(CPACK_RPM_EXCLUDE_FROM_AUTO_FILELIST
    /usr
    /usr/bin
    /usr/share
    /usr/share/man
)

set(CMAKE_MODULE_PATH "${CMAKE_SOURCE_DIR}/cmake")

find_package(Go 1.1 REQUIRED)
find_package(Git REQUIRED)
find_package(Protobuf 2.3 QUIET)
set(CPACK_PACKAGE_FILE_NAME ${CMAKE_PROJECT_NAME}-${CPACK_PACKAGE_VERSION_MAJOR}_${CPACK_PACKAGE_VERSION_MINOR}_${CPACK_PACKAGE_VERSION_PATCH}-${GO_PLATFORM}-${GO_ARCH})
set(CMAKE_INSTALL_PREFIX ${CMAKE_PROJECT_NAME})
set(PROJECT_PATH "${CMAKE_BINARY_DIR}/heka")
set(HEKA_PATH "${PROJECT_PATH}/src/github.com/mozilla-services/heka")
set(LUA_INCLUDE_PATH "${PROJECT_PATH}/include")
set(LUA_LIB_PATH "${PROJECT_PATH}/lib")
set(HEKA_EXE "${PROJECT_PATH}/bin/hekad${CMAKE_EXECUTABLE_SUFFIX}")
set(FLOOD_EXE "${PROJECT_PATH}/bin/heka-flood${CMAKE_EXECUTABLE_SUFFIX}")
set(SBMGR_EXE "${PROJECT_PATH}/bin/heka-sbmgr${CMAKE_EXECUTABLE_SUFFIX}")
set(SBMGRLOAD_EXE "${PROJECT_PATH}/bin/heka-sbmgrload${CMAKE_EXECUTABLE_SUFFIX}")
set(INJECT_EXE "${PROJECT_PATH}/bin/heka-inject${CMAKE_EXECUTABLE_SUFFIX}")

option(INCLUDE_SANDBOX "Include Lua sandbox" on)
option(INCLUDE_MOZSVC "Include the Mozilla services plugins" on)

option(BENCHMARK "Enable the benchmark tests" off)
if (BENCHMARK)
    set(BENCHMARK_FLAG -bench .)
endif()

set(INCLUDE_DOCUMENTATION true)
find_program(SPHINX_BUILD_EXECUTABLE sphinx-build PATH_SUFFIXES bin)
if (NOT SPHINX_BUILD_EXECUTABLE)
    message(STATUS "sphinx-build was not found, the documentation will not be generated.")
    set(INCLUDE_DOCUMENTATION false)
endif()

if (MINGW)
    set(ADDRESS_MODEL 64)
    if (GO_ARCH STREQUAL "386")
        set(ADDRESS_MODEL 32)
    endif()
    set(CPACK_GENERATOR "ZIP")
elseif(APPLE)
    set(CPACK_GENERATOR "PackageMaker")
    set(LDFLAGS "-ldflags=\"-linkmode=external\"")
elseif(UNIX)
    set(CPACK_GENERATOR "TGZ")
    
    find_program(DPKG_EXECUTABLE dpkg PATH_SUFFIXES bin)

    find_program(RPMBUILD_EXECUTABLE rpmbuild PATH_SUFFIXES bin)
    if (RPMBUILD_EXECUTABLE)
        set(CPACK_GENERATOR ${CPACK_GENERATOR} "RPM")
        set(CPACK_RPM_PACKAGE_LICENSE "MPLv2.0")
    endif()
endif()

include(CTest)
include(CPack)
include(externals)
include(mocks)

execute_process(COMMAND "${GIT_EXECUTABLE}" submodule update --init --recursive
WORKING_DIRECTORY "${CMAKE_SOURCE_DIR}"
RESULT_VARIABLE error_code
)
if(error_code)
    message(FATAL_ERROR "Failed to init Heka submodules")
endif()

configure_file("${CMAKE_SOURCE_DIR}/sandbox/lua/lua_sandbox.go.in" "${HEKA_PATH}/sandbox/lua/lua_sandbox.go" @ONLY)

if (EXISTS "${CMAKE_BINARY_DIR}/plugin_loader.go")
    configure_file("${CMAKE_BINARY_DIR}/plugin_loader.go" "${HEKA_PATH}/cmd/hekad/plugin_loader.go" COPYONLY)
endif()

add_custom_target(clean-heka
COMMAND ${CMAKE_COMMAND} -E remove_directory "${HEKA_PATH}"
COMMAND ${CMAKE_COMMAND} -E remove "${HEKA_EXE}" "${FLOOD_EXE}" "${SBMGR_EXE}" "${SBMGRLOAD_EXE}" "${INJECT_EXE}"
COMMAND ${CMAKE_COMMAND} ..
COMMENT "Resynchronizing the Go workspace with the Heka repository"
)

set(MESSAGE_PROTO_OUT "${CMAKE_SOURCE_DIR}/message/message.pb.go")
add_custom_command(
OUTPUT ${MESSAGE_PROTO_OUT}
COMMAND ${CMAKE_COMMAND} -DSRC_DIR="${CMAKE_SOURCE_DIR}" -DPROTOBUF_EXECUTABLE="${PROTOBUF_EXECUTABLE}" -P "${CMAKE_SOURCE_DIR}/cmake/message_proto.cmake"
DEPENDS "${CMAKE_SOURCE_DIR}/message/message.proto"
WORKING_DIRECTORY "${CMAKE_SOURCE_DIR}/message"
COMMENT "Built ${MESSAGE_PROTO_OUT}"
)

if(INCLUDE_SANDBOX)
set(COPY_SANDBOX COMMAND ${CMAKE_COMMAND} -E copy_directory "${CMAKE_SOURCE_DIR}/sandbox" "${HEKA_PATH}/sandbox")
endif()
add_custom_target(heka_source ALL
COMMAND ${CMAKE_COMMAND} -E copy_directory "${CMAKE_SOURCE_DIR}/client" "${HEKA_PATH}/client"
COMMAND ${CMAKE_COMMAND} -E copy_directory "${CMAKE_SOURCE_DIR}/cmd" "${HEKA_PATH}/cmd"
COMMAND ${CMAKE_COMMAND} -E copy_directory "${CMAKE_SOURCE_DIR}/docs" "${HEKA_PATH}/docs"
COMMAND ${CMAKE_COMMAND} -E copy_directory "${CMAKE_SOURCE_DIR}/examples" "${HEKA_PATH}/examples"
COMMAND ${CMAKE_COMMAND} -E copy_directory "${CMAKE_SOURCE_DIR}/message" "${HEKA_PATH}/message"
COMMAND ${CMAKE_COMMAND} -E copy_directory "${CMAKE_SOURCE_DIR}/pipeline" "${HEKA_PATH}/pipeline"
COMMAND ${CMAKE_COMMAND} -E copy_directory "${CMAKE_SOURCE_DIR}/plugins" "${HEKA_PATH}/plugins"
${COPY_SANDBOX}
DEPENDS ${SANDBOX_PACKAGE} GoPackages ${MESSAGE_PROTO_OUT}
)

add_custom_target(message_matcher_parser ALL
COMMAND ${GO_EXECUTABLE} tool yacc -l=false -o=message_matcher_parser.go message_matcher_parser.y
DEPENDS heka_source
WORKING_DIRECTORY "${HEKA_PATH}/message"
)
add_dependencies(mocks message_matcher_parser)

add_custom_target(hekad ALL 
${GO_EXECUTABLE} install ${LDFLAGS} github.com/mozilla-services/heka/cmd/hekad
DEPENDS mocks)

if (INCLUDE_DOCUMENTATION)
    add_custom_target(docs
    COMMAND ${SPHINX_BUILD_EXECUTABLE} -b html -d build/doctrees source build/html
    COMMAND ${SPHINX_BUILD_EXECUTABLE} -b man -d build/doctrees source build/man
    WORKING_DIRECTORY "${HEKA_PATH}/docs"
    DEPENDS heka_source
    COMMENT "Built Heka user/developer documentation")

    install(CODE "execute_process(COMMAND ${CMAKE_BUILD_TOOL} docs)") # always force the docs to rebuild before installing/packaging
    install(DIRECTORY "${HEKA_PATH}/docs/build/man/" DESTINATION share/man/man1 FILES_MATCHING PATTERN "*.1")
    install(DIRECTORY "${HEKA_PATH}/docs/build/man/" DESTINATION share/man/man5 FILES_MATCHING PATTERN "*.5")
endif()

if (MINGW AND INCLUDE_SANDBOX)
    set(LUA_DLL_PATH "${PROJECT_PATH}/lib/liblua.dll")
    set(LPEG_DLL_PATH "${PROJECT_PATH}/lib/liblpeg.dll")
    set(CJSON_DLL_PATH "${PROJECT_PATH}/lib/libcjson.dll")
    set(LUASANDBOX_DLL_PATH "${PROJECT_PATH}/lib/libluasandbox.dll")
    install(FILES ${LUA_DLL_PATH} ${LPEG_DLL_PATH} ${CJSON_DLL_PATH} ${LUASANDBOX_DLL_PATH} DESTINATION bin)
    add_custom_command(TARGET heka_source POST_BUILD
    COMMAND ${CMAKE_COMMAND} -E copy_directory "${PROJECT_PATH}/lib/" "${HEKA_PATH}"
    COMMAND ${CMAKE_COMMAND} -E copy_directory "${PROJECT_PATH}/lib/" "${HEKA_PATH}/pipeline"
    COMMAND ${CMAKE_COMMAND} -E copy_directory "${PROJECT_PATH}/lib/" "${HEKA_PATH}/plugins"
    COMMAND ${CMAKE_COMMAND} -E copy_directory "${PROJECT_PATH}/lib/" "${HEKA_PATH}/sandbox/lua"
    COMMENT "Install dll's for the mock generation and unit tests")
endif()

install(PROGRAMS "${HEKA_EXE}" DESTINATION bin)

add_custom_target(flood ALL 
${GO_EXECUTABLE} install github.com/mozilla-services/heka/cmd/heka-flood
DEPENDS hekad
WORKING_DIRECTORY ${CMAKE_SOURCE_DIR})

install(PROGRAMS "${FLOOD_EXE}" DESTINATION bin)

add_custom_target(inject ALL 
${GO_EXECUTABLE} install github.com/mozilla-services/heka/cmd/heka-inject
DEPENDS hekad
WORKING_DIRECTORY ${CMAKE_SOURCE_DIR})

install(PROGRAMS "${INJECT_EXE}" DESTINATION bin)

add_custom_target(sbmgr ALL 
${GO_EXECUTABLE} install github.com/mozilla-services/heka/cmd/heka-sbmgr
DEPENDS hekad)

install(PROGRAMS "${SBMGR_EXE}" DESTINATION bin)

add_custom_target(sbmgrload ALL 
${GO_EXECUTABLE} install github.com/mozilla-services/heka/cmd/heka-sbmgrload
DEPENDS hekad)

if (UNIX AND DPKG_EXECUTABLE)
    execute_process(COMMAND "${DPKG_EXECUTABLE}" --print-architecture
    OUTPUT_VARIABLE CPACK_DEBIAN_PACKAGE_ARCHITECTURE
    OUTPUT_STRIP_TRAILING_WHITESPACE)

    set(CPACK_DEBIAN_PACKAGE_DEPENDS "libc6 (>= 2.14)")
    get_filename_component(CMAKE_BIN_DIR ${CMAKE_COMMAND} PATH)
    set(CPACK_COMMAND ${CMAKE_BIN_DIR}/cpack)
    
    add_custom_target(deb
    COMMAND ${CMAKE_COMMAND} -E cmake_echo_color --switch=$(COLOR) --cyan "Run CPack packaging tool..."
    COMMAND ${CPACK_COMMAND} -G DEB -D CPACK_PACKAGE_FILE_NAME="${CMAKE_PROJECT_NAME}_${CPACK_PACKAGE_VERSION_MAJOR}.${CPACK_PACKAGE_VERSION_MINOR}.${CPACK_PACKAGE_VERSION_PATCH}_${CPACK_DEBIAN_PACKAGE_ARCHITECTURE}" --config ./CPackConfig.cmake
    DEPENDS hekad
    COMMENT "Custom deb target")
endif()

add_test(message ${GO_EXECUTABLE} test ${LDFLAGS} ${BENCHMARK_FLAG} github.com/mozilla-services/heka/message)
add_test(pipeline ${GO_EXECUTABLE} test ${LDFLAGS} ${BENCHMARK_FLAG} github.com/mozilla-services/heka/pipeline)
add_test(plugins ${GO_EXECUTABLE} test ${LDFLAGS} ${BENCHMARK_FLAG} github.com/mozilla-services/heka/plugins)
add_test(plugins/amqp ${GO_EXECUTABLE} test ${LDFLAGS} ${BENCHMARK_FLAG} github.com/mozilla-services/heka/plugins/amqp)
add_test(plugins/dasher ${GO_EXECUTABLE} test ${LDFLAGS} ${BENCHMARK_FLAG} github.com/mozilla-services/heka/plugins/dasher)
add_test(plugins/elasticsearch ${GO_EXECUTABLE} test ${LDFLAGS} ${BENCHMARK_FLAG} github.com/mozilla-services/heka/plugins/elasticsearch)
add_test(plugins/file ${GO_EXECUTABLE} test ${LDFLAGS} ${BENCHMARK_FLAG} github.com/mozilla-services/heka/plugins/file)
add_test(plugins/graphite ${GO_EXECUTABLE} test ${LDFLAGS} ${BENCHMARK_FLAG} github.com/mozilla-services/heka/plugins/graphite)
add_test(plugins/http ${GO_EXECUTABLE} test ${LDFLAGS} ${BENCHMARK_FLAG} github.com/mozilla-services/heka/plugins/http)
add_test(plugins/payload ${GO_EXECUTABLE} test ${LDFLAGS} ${BENCHMARK_FLAG} github.com/mozilla-services/heka/plugins/payload)
add_test(plugins/process ${GO_EXECUTABLE} test ${LDFLAGS} ${BENCHMARK_FLAG} github.com/mozilla-services/heka/plugins/process)
add_test(plugins/smtp ${GO_EXECUTABLE} test ${LDFLAGS} ${BENCHMARK_FLAG} github.com/mozilla-services/heka/plugins/smtp)
add_test(plugins/statsd ${GO_EXECUTABLE} test ${LDFLAGS} ${BENCHMARK_FLAG} github.com/mozilla-services/heka/plugins/statsd)
add_test(plugins/tcp ${GO_EXECUTABLE} test ${LDFLAGS} ${BENCHMARK_FLAG} github.com/mozilla-services/heka/plugins/tcp)
add_test(plugins/udp ${GO_EXECUTABLE} test ${LDFLAGS} ${BENCHMARK_FLAG} github.com/mozilla-services/heka/plugins/udp)
if(INCLUDE_SANDBOX)
	add_test(sandbox ${GO_EXECUTABLE} test ${LDFLAGS} ${BENCHMARK_FLAG} github.com/mozilla-services/heka/sandbox/lua)
	add_test(sandbox_plugins ${GO_EXECUTABLE} test ${LDFLAGS} ${BENCHMARK_FLAG} github.com/mozilla-services/heka/sandbox/plugins)
endif()
if (INCLUDE_MOZSVC)
    add_test(mozsvc ${GO_EXECUTABLE} test ${BENCHMARK_FLAG} github.com/mozilla-services/heka-mozsvc-plugins)
endif()

install(FILES "${HEKA_PATH}/cmd/heka-sbmgr/heka-sbmgr.toml" "${HEKA_PATH}/cmd/sbmgr/hekad.toml.sbmgr" DESTINATION bin)
if(INCLUDE_SANDBOX)
install(FILES "${HEKA_PATH}/sandbox/lua/testsupport/hekabench_cbuf_counter.lua" DESTINATION bin)
endif()
install(FILES "${CMAKE_SOURCE_DIR}/LICENSE.txt" DESTINATION "share/${CMAKE_PROJECT_NAME}")
install(DIRECTORY "${CMAKE_SOURCE_DIR}/dasher" DESTINATION "share/${CMAKE_PROJECT_NAME}")
install(DIRECTORY "${PROJECT_PATH}/modules/" DESTINATION "share/${CMAKE_PROJECT_NAME}/lua_modules")
install(DIRECTORY "${HEKA_PATH}/sandbox/lua/filters/" DESTINATION "share/${CMAKE_PROJECT_NAME}/lua_filters")
