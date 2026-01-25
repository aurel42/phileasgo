.PHONY: all build test clean vendor

APP_NAME=phileasgo
GUI_NAME=phileasgui
CMD_PATH=./cmd/phileasgo
WEB_PATH=./internal/ui/web

all: build-web test build build-gui

build: build-web build-app build-gui

build-app: pkg/geo/countries.geojson
	powershell -ExecutionPolicy Bypass -File scripts/copy_simconnect.ps1
	go build -o $(APP_NAME).exe $(CMD_PATH)

build-gui: cmd/phileasgui/rsrc_windows_amd64.syso
	go build -ldflags="-H windowsgui" -o $(GUI_NAME).exe ./cmd/phileasgui

cmd/phileasgui/rsrc_windows_amd64.syso: cmd/phileasgui/winres/winres.json $(wildcard data/appicons/*.png)
	cd cmd/phileasgui && go-winres make --in winres/winres.json --out .
	cd cmd/phileasgui && mv ._windows_amd64.syso rsrc_windows_amd64.syso
	cd cmd/phileasgui && mv ._windows_386.syso rsrc_windows_386.syso

pkg/geo/countries.geojson:
	powershell -ExecutionPolicy Bypass -File cmd/slim_geojson/download.ps1

build-web:
	cd $(WEB_PATH) && npm install && npm run build
	powershell -Command "Copy-Item -Path data\\icons -Destination internal\\ui\\dist\\icons -Recurse -Force"

run: build
	./$(APP_NAME).exe

test: lint unit-test

lint:
	golangci-lint run
	go vet ./...

unit-test:
	go test ./...

vendor:
	go mod vendor

clean-all: clean clean-db clean-logs

clean:
	powershell -Command "if (Test-Path internal\\ui\\dist) { Remove-Item -Recurse -Force internal\\ui\\dist }"
	powershell -Command "if (Test-Path $(APP_NAME).exe) { Remove-Item -Force $(APP_NAME).exe }"
	powershell -Command "if (Test-Path $(GUI_NAME).exe) { Remove-Item -Force $(GUI_NAME).exe }"
	powershell -Command "Get-ChildItem -Path cmd\\phileasgui -Filter *.syso | Remove-Item -Force"

clean-db:
	powershell -Command "if (Test-Path data\\phileas.db) { Remove-Item -Force data\\phileas.db }"

clean-logs:
	powershell -Command "if (Test-Path logs\\server.log) { Remove-Item -Force logs\\server.log }"
	powershell -Command "if (Test-Path logs\\requests.log) { Remove-Item -Force logs\\requests.log }"

VERSION=$(shell grep "const Version =" pkg/version/version.go | sed 's/.*"\(.*\)".*/\1/')
PLATFORM=windows-x64

release-binary: clean build
	powershell -Command "if (Test-Path release) { Remove-Item -Recurse -Force release }"
	powershell -Command "New-Item -ItemType Directory -Path release | Out-Null"
	powershell -Command "Compress-Archive -Path $(APP_NAME).exe, $(GUI_NAME).exe, README.md, HISTORY.md, .env.template, install.ps1, configs -DestinationPath release/$(APP_NAME)-$(VERSION)-$(PLATFORM).zip -Force"
	@echo Release created: release/$(APP_NAME)-$(VERSION)-$(PLATFORM).zip
