# jrandr

Dynamically configure X11 outputs using RandR. 

## What is it?

jrandr listens to external monitor hotplug events and automatically
configure X11 outputs according to the specified configuration file.

If your are using a laptop, it can also receive lid open/close events
to automatically enable/disable the laptop panel output.

This program is useful if you are using an X11 window manager such as
i3wm which does not perform automatic output configuration and expects
the user to run xrandr manually instead.

I developped this program on my free time and for my own use. I hope it
can be useful to others but you should expect bugs.

## Usage

When invoked without arguments, `jrandr` displays the list of outputs, how
they're configured and what monitor is currently connected (similarly to
what xrandr already does).

Example: 

```
$ ./jrandr 
Outputs:
  [43] eDP-1: CRTC=0 connected, best mode: 1920x1080 (4a), Monitor=CMN-5352-0
  [44] HDMI-1: CRTC=40 connected, best mode: 3440x1440 (9c), current mode: 3440x1440 (9c), position: 0+0, Monitor=SAM-3655-1129860428, primary
  [45] DP-1: CRTC=0 disconnected
  [46] HDMI-2: CRTC=0 disconnected
  [47] DP-2: CRTC=0 disconnected
  [48] HDMI-3: CRTC=0 disconnected

```

When providing a configuration file as an argument on the command line,
`jrandr` will start listening and process events until you tell it to
stop.

Example:

```
$ ./jrandr config.yml
2023/01/16 09:52:40 Waiting 1 seconds before starting
2023/01/16 09:52:41 Found matching setup from config: office
2023/01/16 09:52:41 Disabling eDP-1 because lid is closed
2023/01/16 09:52:41 Display rect: left 0 right 3440 top 0 bottom 1440
2023/01/16 09:52:41 Reconfiguring CRTC=40:
2023/01/16 09:52:41   mode 9c -> 9c
2023/01/16 09:52:41   position 1920+0 -> 0+0
2023/01/16 09:52:41 Setting screen size 3440x1440 (910x381mm)
2023/01/16 09:52:41 Setting primary output: HDMI-1
2023/01/16 09:52:42 Processing events...
2023/01/16 09:52:42 output change: eDP-1 connected=true
2023/01/16 09:52:42 Found matching setup from config: office
2023/01/16 09:52:42 Display rect: left 0 right 3440 top 0 bottom 1440
```

If you use i3, you can run it in the background when i3 starts by adding
this line to `~/.config/i3/config`:

```
exec --no-startup-id jrandr ~/.config/i3/jrandr.yml > ~/.config/i3/jrandr.log 2>&1
```

## Configuration

Example configuration:

```
wait: 1s
default_extend_direction: right
background_command: feh --no-fehbg --bg-scale --no-xinerama /usr/share/wallpapers/Next/contents/images_dark/5120x2880.png
setups:
  office:
    outputs:
    - monitor: CMN-5352-0
      position:
        x: 0
        y: 1080
      primary: false
      disable_on_lid_close: true
    - monitor: SAM-3655-1129860428
      position:
        x: 1920
        y: 0
      primary: true
```

- `wait`: Backoff time before `jrandr` starts autoconfiguring outputs
  after startup. If `jrandr` is started by your window manager, you
  might need this to make sure the window manager startup.
  finishes before `jrandr` starts changing things around.
  (duration as parsed by golang's `time.ParseDuration`)

- `default_extend_direction`: Direction in which newly connected
  monitors will extend the desktop. ('left', 'right', 'top', 'bottom').
  Only used when no "setup" is currently selected (see below).

- `background_command`: Command that will be executed each time `jrandr`
  changes the screen configuration. For example to reset the
  desktop background image if needed.

- `setups`: Dictionary of setup configs. Each setup config represents
  a particular disposition of outputs. `jrandr` will try to match 
  the currently connected monitors with the ones declared in these
  setups configs to determine how to configure the outputs.

  - `outputs`: List of outputs to configure when the current setup
    config is selected.

    - `monitor`: The identifier of the connected monitor. The
      identifiers are determined using the monitor EDID and are
      (in theory) unique. (You can find out the identifier of the
      currently connected monitors by invoking `jrandr` without
      arguments).

    - `position`: Position in pixels of this output relative to others:
      - `x`: Horizontal position.
      - `y`: Vertical position.

    - `primary`: Whether to set this output as the primary output or not
      when switching to this setup.

    - `disable_on_lid_close`: Disable this output when the laptop lid is
      closed.

## Build

```
go build
```
