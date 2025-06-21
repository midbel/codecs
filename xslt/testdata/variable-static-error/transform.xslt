<?xml version="1.0" encoding="UTF-8"?>

<xsl:stylesheet version="3.0" 
	xmlns:xsl="http://www.w3.org/1999/XSL/Transform">
	<xsl:output method="xml" indent="yes"/>
	<xsl:variable name="foo" select="'foobar'" static="yes"/>
	<xsl:template match="/">
		<item>
			<xsl:value-of select="$foo"/>
		</item>
	</xsl:template>
</xsl:stylesheet>