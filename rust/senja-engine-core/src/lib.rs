//! Pure state rules for the future resident Windows engine.

pub const STYLE_GLASS: i32 = 1;
pub const STYLE_SOLID: i32 = 4;

#[derive(Clone, Debug, PartialEq)]
pub struct Config {
    pub style: i32,
    pub startup: bool,
    pub auto_update: bool,
    pub auto_install: bool,
    pub blur_radius: f32,
    pub rounded: bool,
}

impl Default for Config {
    fn default() -> Self {
        Self {
            style: STYLE_GLASS,
            startup: false,
            auto_update: true,
            auto_install: false,
            blur_radius: 18.0,
            rounded: true,
        }
    }
}

impl Config {
    /// Returns the loaded config and whether the legacy file needs repair.
    pub fn parse(input: &str) -> (Self, bool) {
        let mut config = Self::default();
        let mut repaired = false;
        for line in input.lines() {
            let line = line.trim();
            if line.is_empty() || line.starts_with([';', '#']) {
                continue;
            }
            let Some((key, value)) = line.split_once('=') else {
                continue;
            };
            let value = value.trim();
            match key.trim() {
                "style" => match value.parse::<i32>() {
                    Ok(0) => {
                        config.style = STYLE_SOLID;
                        repaired = true;
                    }
                    Ok(style @ 1..=4) => config.style = style,
                    Ok(_) => {
                        config.style = STYLE_GLASS;
                        repaired = true;
                    }
                    Err(_) => repaired = true,
                },
                "startup" => config.startup = enabled(value),
                "auto_update" => config.auto_update = enabled(value),
                "auto_install" => config.auto_install = enabled(value),
                "blur_radius" => {
                    if let Ok(radius) = value.parse::<f32>()
                        && (1.0..=50.0).contains(&radius)
                    {
                        config.blur_radius = radius;
                    }
                }
                "rounded" => config.rounded = enabled(value),
                _ => {}
            }
        }
        (config, repaired)
    }

    pub fn serialize(&self) -> String {
        let style = if (0..=STYLE_SOLID).contains(&self.style) {
            self.style
        } else {
            STYLE_GLASS
        };
        let blur = if self.blur_radius <= 0.0 {
            18.0
        } else {
            self.blur_radius
        };
        format!(
            "style={style}\nstartup={}\nauto_update={}\nauto_install={}\nblur_radius={blur:.1}\nrounded={}\n",
            i32::from(self.startup),
            i32::from(self.auto_update),
            i32::from(self.auto_install),
            i32::from(self.rounded)
        )
    }
}

fn enabled(value: &str) -> bool {
    value == "1" || value.eq_ignore_ascii_case("true")
}

#[cfg(test)]
mod tests {
    use super::*;

    #[test]
    fn defaults_and_legacy_styles_match_go() {
        assert_eq!(Config::parse("").0, Config::default());
        assert_eq!(
            Config::parse("style=0\n"),
            (
                Config {
                    style: STYLE_SOLID,
                    ..Config::default()
                },
                true
            )
        );
        assert_eq!(Config::parse("style=99\n"), (Config::default(), true));
        assert_eq!(
            Config::parse("style=3\nstyle=oops\n"),
            (
                Config {
                    style: 3,
                    ..Config::default()
                },
                true
            )
        );
    }

    #[test]
    fn settings_round_trip_and_ignore_bad_values() {
        let input = "# comment\nstyle=2\nstartup=TRUE\nauto_update=0\nauto_install=1\nblur_radius=24.5\nrounded=false\nunknown=ignored\n";
        let (config, repaired) = Config::parse(input);
        assert!(!repaired);
        assert_eq!(config.style, 2);
        assert!(config.startup && config.auto_install);
        assert!(!config.auto_update && !config.rounded);
        assert_eq!(config.blur_radius, 24.5);
        assert_eq!(Config::parse(&config.serialize()), (config, false));
        assert_eq!(
            Config::parse("blur_radius=NaN\nblur_radius=51\n")
                .0
                .blur_radius,
            18.0
        );
    }
}
